# telesma-app/hid

[![Go Reference](https://pkg.go.dev/badge/github.com/telesma-app/hid.svg)](https://pkg.go.dev/github.com/telesma-app/hid)
[![Go](https://github.com/telesma-app/hid/actions/workflows/go.yml/badge.svg)](https://github.com/telesma-app/hid/actions/workflows/go.yml)

`telesma-app/hid` is a cgo-free Go library for discovering and communicating with HID devices on Windows, macOS, and Linux. It uses native operating-system facilities and requires neither `libhidapi` nor a C toolchain.

The library was created primarily as the HID backend for [`telesma-app/ctap`](https://github.com/telesma-app/ctap), but it is protocol-agnostic and can be used independently with other HID devices. The module is currently pre-v1, so its API may continue to evolve between minor releases.

## Supported platforms

| Capability | Windows | macOS | Linux |
| --- | --- | --- | --- |
| Enumeration and filtering | Yes | Yes | Yes |
| Connection events | Yes | Yes | Yes |
| Input/output reports | Yes | Yes | Yes |
| Feature reports | Yes | Yes | Yes |
| Context-aware reads and writes | Yes | Yes | Yes |
| Configurable read timeout | Yes | — | — |
| Native backend | HID, SetupAPI, Configuration Manager | IOKit and Core Foundation via `purego` | `hidraw`, sysfs, and kernel uevents |

## Installation

The module requires Go 1.27 or newer.

```sh
go get github.com/telesma-app/hid
```

## Usage

`Enumerate` returns a Go iterator. Filters are exact matches and can be combined; this example selects the FIDO HID usage collection used by `telesma-app`:

```go
for info, err := range hid.Enumerate(
	hid.WithUsagePage(0xf1d0),
	hid.WithUsage(0x01),
) {
	if err != nil {
		log.Printf("enumerate HID: %v", err)
	}
	if info == nil {
		continue
	}

	log.Printf("path=%q", info.Path)
	if info.ProductStr.Value != nil {
		log.Printf("product=%q", *info.ProductStr.Value)
	}
}
```

`VendorID` and `ProductID` are `Descriptor[uint16]`; `MfrStr`, `ProductStr`, and `SerialNbr` are `Descriptor[string]`. Each carries only `Value *T` and `Err error`:

- A non-nil `Value` is a successful read, including a zero ID or an empty string.
- A non-nil `Err` is a read or conversion failure; the original Go error is retained.
- Both nil means the platform did not provide a value; descriptor presence is unknown.

The caller owns serialization of descriptor errors; `Err` is omitted from JSON.

Descriptor failures do not hide a device from enumeration or events; filters
match only successfully read values. Enumeration can return both a device and
an error for incomplete discovery metadata; callers can still use the returned
fields. The same error is retained as `MetadataErr` in watcher snapshots and events.

Pass `DeviceInfo.Path` to `OpenPath` to get a device with `Read`, `Write`, `SendFeatureReport`, `GetFeatureReport`, and `Close`. Output and feature-report buffers begin with the report ID; use `0` for an unnumbered report. Higher-level framing, such as CTAPHID, is intentionally left to packages such as [`telesma-app/ctap`](https://github.com/telesma-app/ctap).

Reads and writes accept a context because they can block. Cancellation is best-effort:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

if _, err := device.Write(ctx, request); err != nil {
	log.Fatal(err)
}
if _, err := device.Read(ctx, response); err != nil {
	log.Fatal(err)
}
```

After a canceled write, do not assume that the report was not sent and do not automatically retry it. The driver or device may finish an in-flight write after `Write` returns `ctx.Err()`.

## Connection events

`Watch` captures every HID device already present in an initial snapshot, then publishes live `connected` and `disconnected` events.

```go
watcher, err := hid.Watch()
if err != nil {
	log.Fatal(err)
}
defer watcher.Close()

for _, device := range watcher.Snapshot().Devices {
	if device.DeviceInfo != nil {
		log.Printf("present: %s", device.DeviceInfo.Path)
	}
}

for event := range watcher.Listen() {
	if event.DeviceInfo != nil {
		log.Printf("%s: %s", event.Type, event.DeviceInfo.Path)
	}
	if event.MetadataErr != nil {
		log.Printf("HID event metadata: %v", event.MetadataErr)
	}
}

if err := watcher.Close(); err != nil {
	log.Printf("HID watcher stopped: %v", err)
}
```

Watch covers all HID devices and should be filtered by the caller. Delivery is ordered and queued, so the channel should be consumed continuously or the watcher closed when it is no longer needed. A non-nil `DeviceEvent.MetadataErr` means that the state change occurred but some metadata may be incomplete. When `Listen` closes unexpectedly, call `Close` to retrieve the terminal watcher error.

## Platform notes

- Device paths are opaque and platform-specific. Optional descriptor availability is recorded explicitly; other metadata remains best-effort.
- Windows reads descriptors through `HidD_GetAttributes` and the HID string APIs. macOS reads IOHID properties; a missing property is unknown, because IOHID does not expose its underlying read outcome.
- Linux reads USB descriptors from sysfs. `HIDName` and `HIDUnique` preserve kernel-provided names and identifiers separately; `HIDUnique` may be a transport address.
- Enumeration and event monitoring do not guarantee I/O access; `OpenPath` remains subject to operating-system, driver, and sandbox policy.
- Reads block by default. A context deadline is portable; `WithReadTimeout` remains available in Windows builds.
- Cancellation is best-effort. Windows requests cancellation of the specific overlapped read or write with `CancelIoEx`. On macOS, canceling a read stops waiting for the next callback report. Linux I/O and an in-flight macOS write may continue in the driver or device after the method returns; operations of the same kind remain serialized until the native call finishes.
- Feature-report methods do not accept a context because the synchronous HID APIs used here do not provide a practical, operation-specific cancellation mechanism.
- On macOS, enumeration and events do not open devices, but opening protected devices for I/O may still be denied by system or sandbox policy.
- On Linux, access to `/dev/hidrawN` depends on udev rules and permissions. A connection event may arrive before the device node and its final permissions are ready.

## Testing

```sh
go test ./...
CGO_ENABLED=0 go test ./...
go vet ./...
```

## License

Licensed under the [Apache License 2.0](LICENSE).
