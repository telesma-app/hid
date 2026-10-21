package hid

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsPartialDescriptorsPreserveUsage(t *testing.T) {
	originalAttributes, originalString := windowsGetAttributes, windowsGetString
	originalPreparsed, originalFree, originalCaps := windowsGetPreparsedData, windowsFreePreparsedData, windowsGetCaps
	t.Cleanup(func() {
		windowsGetAttributes, windowsGetString = originalAttributes, originalString
		windowsGetPreparsedData, windowsFreePreparsedData, windowsGetCaps = originalPreparsed, originalFree, originalCaps
	})
	windowsGetAttributes = func(windows.Handle) (*_HIDD_ATTRIBUTES, error) {
		return &_HIDD_ATTRIBUTES{VendorID: 0x1050, ProductID: 0x0407}, nil
	}
	windowsGetString = func(_ windows.Handle, proc *windows.LazyProc) (string, error) {
		switch proc {
		case procHidD_GetManufacturerString:
			return "", windows.ERROR_GEN_FAILURE
		case procHidD_GetProductString:
			return "Security Key", nil
		case procHidD_GetSerialNumberString:
			return "", nil
		default:
			t.Fatalf("unexpected HID string procedure: %v", proc)
			return "", nil
		}
	}
	windowsGetPreparsedData = func(windows.Handle) (_PHIDP_PREPARSED_DATA, error) { return 1, nil }
	freed := 0
	windowsFreePreparsedData = func(_PHIDP_PREPARSED_DATA) error { freed++; return nil }
	windowsGetCaps = func(_PHIDP_PREPARSED_DATA) (*_HIDP_CAPS, error) { return &_HIDP_CAPS{UsagePage: 0xf1d0, Usage: 1}, nil }
	var info DeviceInfo
	if err := fillWindowsDeviceInfo(&info, 1); err != nil {
		t.Fatal(err)
	}
	if info.UsagePage != 0xf1d0 || info.Usage != 1 || freed != 1 || *info.VendorID.Value != 0x1050 || *info.ProductID.Value != 0x0407 {
		t.Fatalf("lost metadata or leaked preparsed data: %#v, frees=%d", info, freed)
	}
	if !errors.Is(info.MfrStr.Err, windows.ERROR_GEN_FAILURE) {
		t.Fatalf("manufacturer = %#v", info.MfrStr)
	}
	if info.ProductStr.Value == nil || *info.ProductStr.Value != "Security Key" || info.SerialNbr.Value == nil || *info.SerialNbr.Value != "" {
		t.Fatalf("other strings = %#v / %#v", info.ProductStr, info.SerialNbr)
	}
	windowsGetAttributes = func(windows.Handle) (*_HIDD_ATTRIBUTES, error) { return nil, windows.ERROR_DEVICE_NOT_CONNECTED }
	info = DeviceInfo{}
	if err := fillWindowsDeviceInfo(&info, 1); err != nil {
		t.Fatal(err)
	}
	if info.VendorID.Err == nil || info.ProductID.Err == nil || info.UsagePage != 0xf1d0 || info.ProductStr.Value == nil || freed != 2 {
		t.Fatalf("attributes failure lost independent observations: %#v", info)
	}
	windowsGetCaps = func(_PHIDP_PREPARSED_DATA) (*_HIDP_CAPS, error) {
		return nil, windows.ERROR_DEVICE_NOT_CONNECTED
	}
	info = DeviceInfo{}
	if err := fillWindowsDeviceInfo(&info, 1); !errors.Is(err, windows.ERROR_DEVICE_NOT_CONNECTED) {
		t.Fatalf("capabilities error = %v", err)
	}
	if info.ProductStr.Value == nil || *info.ProductStr.Value != "Security Key" || freed != 3 {
		t.Fatalf("capabilities failure discarded descriptors or leaked preparsed data: %#v, frees=%d", info, freed)
	}
}

func TestGetHidGuid(t *testing.T) {
	guid, err := getHidGuid()
	if err != nil {
		t.Fatal(err)
	}

	if guid == nil {
		t.Fatal("getHidGuid returned nil")
	}
	if *guid == (windows.GUID{}) {
		t.Fatal("getHidGuid returned an empty GUID")
	}
}

func TestReadCancellationCallsCancelIoEx(t *testing.T) {
	originalReadFile := windowsReadFile
	originalCancelIoEx := windowsCancelIoEx
	t.Cleanup(func() {
		windowsReadFile = originalReadFile
		windowsCancelIoEx = originalCancelIoEx
	})

	started := make(chan struct{})
	release := make(chan struct{})
	cancelCalled := make(chan *windows.Overlapped, 1)
	windowsReadFile = func(_ windows.Handle, _ []byte, _ *uint32, _ *windows.Overlapped) error {
		close(started)
		<-release
		return windows.ERROR_OPERATION_ABORTED
	}
	windowsCancelIoEx = func(_ windows.Handle, overlapped *windows.Overlapped) error {
		cancelCalled <- overlapped
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	device := &Device{hFile: 1, inputReportByteLength: 64, readTimeout: windows.INFINITE}
	done := make(chan error, 1)
	go func() {
		_, err := device.Read(ctx, make([]byte, 64))
		done <- err
	}()

	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not return after cancellation")
	}

	select {
	case overlapped := <-cancelCalled:
		if overlapped == nil {
			t.Fatal("CancelIoEx was called without the operation's OVERLAPPED")
		}
	case <-time.After(time.Second):
		t.Fatal("CancelIoEx was not called")
	}
	close(release)
}
