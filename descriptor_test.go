package hid

import (
	"errors"
	"testing"
)

func TestDescriptorFiltersRequireReadValue(t *testing.T) {
	for _, option := range []EnumerateOption{
		WithVendorID(0), WithProductID(0), WithManufacturerString(""),
		WithProductString(""), WithSerialNumber(""),
	} {
		opts := newEnumerateOptions([]EnumerateOption{option})
		if opts.match(&DeviceInfo{}) {
			t.Fatal("unknown descriptor matched a zero-value filter")
		}

		info := DeviceInfo{
			VendorID:   Descriptor[uint16]{Value: new(uint16)},
			ProductID:  Descriptor[uint16]{Value: new(uint16)},
			MfrStr:     Descriptor[string]{Value: new(string)},
			ProductStr: Descriptor[string]{Value: new(string)},
			SerialNbr:  Descriptor[string]{Value: new(string)},
		}
		if !opts.match(&info) {
			t.Fatal("read zero value did not match filter")
		}
	}

	info := DeviceInfo{UsagePage: 0xf1d0, Usage: 1, SerialNbr: Descriptor[string]{Err: errors.New("unavailable")}}

	opts := newEnumerateOptions([]EnumerateOption{WithUsagePage(0xf1d0), WithUsage(1)})
	if !opts.match(&info) {
		t.Fatal("optional descriptor failure hid a FIDO device")
	}
}
