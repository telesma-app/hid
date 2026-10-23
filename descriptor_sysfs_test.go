package hid

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func sysfsFixture(t *testing.T, bus string) (usbPath, hidPath string) {
	t.Helper()
	usbPath = t.TempDir()
	hidPath = filepath.Join(usbPath, "interface", "hid")
	if err := os.MkdirAll(hidPath, 0700); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		filepath.Join(usbPath, "uevent"):            "DEVTYPE=usb_device\n",
		filepath.Join(usbPath, "product"):           "Actual USB product\n",
		filepath.Join(usbPath, "manufacturer"):      "Manufacturer\n",
		filepath.Join(hidPath, "uevent"):            "HID_ID=" + bus + ":00001050:00000407\nHID_NAME=Synthetic HID name\nHID_UNIQ=not-a-serial\n",
		filepath.Join(hidPath, "report_descriptor"): "\x06\xd0\xf1\x09\x01",
	} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return usbPath, hidPath
}

func TestSysfsPartialDescriptors(t *testing.T) {
	usbPath, hidPath := sysfsFixture(t, "0003")
	// A directory in place of an attribute produces a real read failure even
	// when tests run as root; other descriptor reads must still succeed.
	if err := os.Remove(filepath.Join(usbPath, "manufacturer")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(usbPath, "manufacturer"), 0700); err != nil {
		t.Fatal(err)
	}
	info, err := readLinuxDeviceInfo("/dev/hidraw-test", hidPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.UsagePage != 0xf1d0 || info.Usage != 1 || info.VendorID.Value == nil || *info.VendorID.Value != 0x1050 {
		t.Fatalf("lost FIDO metadata: %#v", info)
	}
	var pathErr *os.PathError
	if !errors.As(info.MfrStr.Err, &pathErr) {
		t.Fatalf("manufacturer = %#v", info.MfrStr)
	}
	if info.ProductStr.Value == nil || *info.ProductStr.Value != "Actual USB product" || info.SerialNbr != (Descriptor[string]{}) {
		t.Fatalf("strings = %#v / %#v", info.ProductStr, info.SerialNbr)
	}
	if err := os.WriteFile(filepath.Join(usbPath, "serial"), []byte("\n"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err = readLinuxDeviceInfo("/dev/hidraw-test", hidPath)
	if err != nil || info.SerialNbr.Value == nil || *info.SerialNbr.Value != "" {
		t.Fatalf("empty serial = %#v, error = %v", info.SerialNbr, err)
	}
}

func TestSysfsDoesNotUseBluetoothAdapterDescriptors(t *testing.T) {
	_, hidPath := sysfsFixture(t, "0005")
	info, err := readLinuxDeviceInfo("/dev/hidraw-test", hidPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.HIDName.Value == nil || *info.HIDName.Value != "Synthetic HID name" || info.HIDUnique.Value == nil || *info.HIDUnique.Value != "not-a-serial" {
		t.Fatalf("lost HID observations: %#v", info)
	}
	if info.MfrStr != (Descriptor[string]{}) || info.ProductStr != (Descriptor[string]{}) || info.SerialNbr != (Descriptor[string]{}) {
		t.Fatalf("used USB adapter or synthetic HID strings: %#v", info)
	}
}

func TestSysfsIDFailurePreservesOtherObservations(t *testing.T) {
	_, hidPath := sysfsFixture(t, "0003")
	if err := os.WriteFile(filepath.Join(hidPath, "uevent"), []byte("HID_ID=0003:00010000:00000407\n"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := readLinuxDeviceInfo("/dev/hidraw-test", hidPath)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(info.VendorID.Err, strconv.ErrRange) {
		t.Fatalf("vendor ID = %#v", info.VendorID)
	}
	if info.ProductID.Value == nil || *info.ProductID.Value != 0x0407 || info.ProductStr.Value == nil || info.UsagePage != 0xf1d0 {
		t.Fatalf("lost independently read metadata: %#v", info)
	}
	if err := os.Remove(filepath.Join(hidPath, "uevent")); err != nil {
		t.Fatal(err)
	}
	info, err = readLinuxDeviceInfo("/dev/hidraw-test", hidPath)
	if err != nil || !errors.Is(info.VendorID.Err, os.ErrNotExist) || info.HIDName.Err == nil || info.HIDUnique.Err == nil || info.UsagePage != 0xf1d0 {
		t.Fatalf("uevent failure = %#v, %v", info, err)
	}
	if info.MfrStr != (Descriptor[string]{}) || info.ProductStr != (Descriptor[string]{}) || info.SerialNbr != (Descriptor[string]{}) {
		t.Fatalf("uevent failure affected unread USB strings: %#v", info)
	}
}

func TestSysfsMissingDevicePreservesPath(t *testing.T) {
	info, err := readLinuxDeviceInfo("/dev/hidraw9", t.TempDir())
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("readLinuxDeviceInfo error = %v, want os.ErrNotExist", err)
	}
	if info == nil || info.Path != "/dev/hidraw9" {
		t.Fatalf("device info = %#v, want the requested path", info)
	}
	if !errors.Is(info.VendorID.Err, os.ErrNotExist) || !errors.Is(info.ProductID.Err, os.ErrNotExist) ||
		!errors.Is(info.HIDName.Err, os.ErrNotExist) || !errors.Is(info.HIDUnique.Err, os.ErrNotExist) {
		t.Fatalf("missing device lost descriptor errors: %#v", info)
	}
}
