package hid

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/telesma-app/hid/reportparser"
)

func readLinuxDeviceInfo(path, sysfsDevicePath string) (*DeviceInfo, error) {
	info := &DeviceInfo{Path: path}

	uevent, err := os.ReadFile(filepath.Join(sysfsDevicePath, "uevent"))

	var usb bool

	if err != nil {
		info.HIDName = Descriptor[string]{Err: err}
		info.HIDUnique = Descriptor[string]{Err: err}
		info.VendorID = Descriptor[uint16]{Err: err}
		info.ProductID = Descriptor[uint16]{Err: err}
	} else {
		usb = fillDeviceInfoFromUevent(info, uevent)
	}

	if usb {
		usbPath, err := linuxUSBDevicePath(sysfsDevicePath)
		if err != nil {
			info.MfrStr = Descriptor[string]{Err: err}
			info.ProductStr = Descriptor[string]{Err: err}
			info.SerialNbr = Descriptor[string]{Err: err}
		} else if usbPath != "" {
			info.MfrStr = readLinuxString(usbPath, "manufacturer")
			info.ProductStr = readLinuxString(usbPath, "product")
			info.SerialNbr = readLinuxString(usbPath, "serial")
		}
	}

	rawDescriptor, err := os.ReadFile(filepath.Join(sysfsDevicePath, "report_descriptor"))
	if err != nil {
		return info, err
	}

	fillDeviceInfoUsage(info, rawDescriptor)

	return info, nil
}

func linuxUSBDevicePath(hidPath string) (string, error) {
	path, err := filepath.EvalSymlinks(hidPath)
	if err != nil {
		return "", err
	}
	// The USB device is two parents above HID; do not walk into a parent hub.
	path = filepath.Dir(filepath.Dir(path))

	uevent, err := os.ReadFile(filepath.Join(path, "uevent"))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(uevent), "\n") {
		if line == "DEVTYPE=usb_device" {
			return path, nil
		}
	}

	return "", nil
}

func readLinuxString(path, name string) Descriptor[string] {
	data, err := os.ReadFile(filepath.Join(path, name))
	if errors.Is(err, os.ErrNotExist) {
		// A missing sysfs attribute does not establish descriptor absence.
		return Descriptor[string]{}
	}

	if err != nil {
		return Descriptor[string]{Err: err}
	}

	return Descriptor[string]{Value: new(strings.TrimSuffix(string(data), "\n"))}
}

func fillDeviceInfoFromUevent(info *DeviceInfo, uevent []byte) (usb bool) {
	for _, line := range strings.Split(string(uevent), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		switch key {
		case "HID_NAME":
			info.HIDName = Descriptor[string]{Value: new(value)}
		case "HID_UNIQ":
			info.HIDUnique = Descriptor[string]{Value: new(value)}
		case "HID_ID":
			parts := strings.Split(value, ":")
			if len(parts) != 3 {
				err := fmt.Errorf("invalid HID_ID %q", value)
				info.VendorID = Descriptor[uint16]{Err: err}
				info.ProductID = Descriptor[uint16]{Err: err}

				continue
			}

			usb = parts[0] == "0003" // BUS_USB; never use a Bluetooth adapter's USB strings.
			info.VendorID = parseLinuxID(parts[1])
			info.ProductID = parseLinuxID(parts[2])
		}
	}

	return usb
}

func parseLinuxID(value string) Descriptor[uint16] {
	n, err := strconv.ParseUint(value, 16, 16)
	if err != nil {
		return Descriptor[uint16]{Err: err}
	}

	return Descriptor[uint16]{Value: new(uint16(n))}
}

func fillDeviceInfoUsage(info *DeviceInfo, rawDescriptor []byte) {
	for _, item := range reportparser.ParseReport(rawDescriptor) {
		switch e := item.(type) {
		case reportparser.UsagePage:
			if info.UsagePage == 0 {
				info.UsagePage = e.Value()
			}
		case reportparser.Usage:
			if info.Usage == 0 {
				info.Usage = e.Value()
			}
		}
	}
}
