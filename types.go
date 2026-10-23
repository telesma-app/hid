package hid

// Descriptor holds a platform observation. A non-nil Value is a successful
// read, including zero IDs and empty strings. Err holds a read failure.
// When neither is set, the platform did not provide a value.
type Descriptor[T any] struct {
	Value *T    `json:"value"`
	Err   error `json:"-"`
}

type DeviceInfo struct {
	Path string // Platform-Specific Device Path
	// VendorID and ProductID may be unavailable on macOS.
	VendorID       Descriptor[uint16]
	ProductID      Descriptor[uint16]
	SerialNbr      Descriptor[string] // Reported serial; not a verified identity
	ReleaseNbr     uint16             // Device Version Number
	MfrStr         Descriptor[string]
	ProductStr     Descriptor[string]
	UsagePage      uint16 // Usage Page for Device/Interface
	Usage          uint16 // Usage for Device/Interface
	InterfaceNbr   int    // USB Interface Number
	InstanceID     string
	ParentDeviceID string
	// HIDName and HIDUnique retain Linux HID_NAME and HID_UNIQ separately from
	// USB descriptors. HIDUnique may be a transport address.
	HIDName   Descriptor[string]
	HIDUnique Descriptor[string]
}

type EnumerateOption func(*enumerateOptions)

type enumerateOptions struct {
	path           *string
	vendorID       *uint16
	productID      *uint16
	serialNbr      *string
	releaseNbr     *uint16
	mfrStr         *string
	productStr     *string
	usagePage      *uint16
	usage          *uint16
	interfaceNbr   *int
	instanceID     *string
	parentDeviceID *string
}

func WithPath(path string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.path = &path
	}
}

func WithVendorID(vendorID uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.vendorID = &vendorID
	}
}

func WithProductID(productID uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.productID = &productID
	}
}

func WithSerialNumber(serialNbr string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.serialNbr = &serialNbr
	}
}

func WithReleaseNumber(releaseNbr uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.releaseNbr = &releaseNbr
	}
}

func WithManufacturerString(mfrStr string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.mfrStr = &mfrStr
	}
}

func WithProductString(productStr string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.productStr = &productStr
	}
}

func WithUsagePage(usagePage uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.usagePage = &usagePage
	}
}

func WithUsage(usage uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.usage = &usage
	}
}

func WithInterfaceNumber(interfaceNbr int) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.interfaceNbr = &interfaceNbr
	}
}

func WithInstanceID(instanceID string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.instanceID = &instanceID
	}
}

func WithParentDeviceID(parentDeviceID string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.parentDeviceID = &parentDeviceID
	}
}

func newEnumerateOptions(options []EnumerateOption) enumerateOptions {
	var opts enumerateOptions
	for _, option := range options {
		option(&opts)
	}

	return opts
}

func (opts enumerateOptions) match(info *DeviceInfo) bool {
	if opts.path != nil && info.Path != *opts.path {
		return false
	}

	if opts.vendorID != nil && (info.VendorID.Value == nil || *info.VendorID.Value != *opts.vendorID) {
		return false
	}

	if opts.productID != nil && (info.ProductID.Value == nil || *info.ProductID.Value != *opts.productID) {
		return false
	}

	if opts.serialNbr != nil && (info.SerialNbr.Value == nil || *info.SerialNbr.Value != *opts.serialNbr) {
		return false
	}

	if opts.releaseNbr != nil && info.ReleaseNbr != *opts.releaseNbr {
		return false
	}

	if opts.mfrStr != nil && (info.MfrStr.Value == nil || *info.MfrStr.Value != *opts.mfrStr) {
		return false
	}

	if opts.productStr != nil && (info.ProductStr.Value == nil || *info.ProductStr.Value != *opts.productStr) {
		return false
	}

	if opts.usagePage != nil && info.UsagePage != *opts.usagePage {
		return false
	}

	if opts.usage != nil && info.Usage != *opts.usage {
		return false
	}

	if opts.interfaceNbr != nil && info.InterfaceNbr != *opts.interfaceNbr {
		return false
	}

	if opts.instanceID != nil && info.InstanceID != *opts.instanceID {
		return false
	}

	if opts.parentDeviceID != nil && info.ParentDeviceID != *opts.parentDeviceID {
		return false
	}

	return true
}
