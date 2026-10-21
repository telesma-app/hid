package hid

import (
	"context"
	"errors"
	"iter"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	linuxHIDRawClassDir = "/sys/class/hidraw"
	linuxDeviceDir      = "/dev"
)

func linuxHIDRawNames() ([]string, error) {
	dir, err := os.Open(linuxHIDRawClassDir)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = dir.Close()
	}()

	return dir.Readdirnames(0)
}

func getLinuxDeviceInfo(name string) (*DeviceInfo, error) {
	name = filepath.Base(name)
	return readLinuxDeviceInfo(filepath.Join(linuxDeviceDir, name), filepath.Join(linuxHIDRawClassDir, name, "device"))
}

func Enumerate(options ...EnumerateOption) iter.Seq2[*DeviceInfo, error] {
	opts := newEnumerateOptions(options)

	return func(yield func(*DeviceInfo, error) bool) {
		names, err := linuxHIDRawNames()
		if err != nil {
			yield(nil, err)
			return
		}

		for _, name := range names {
			info, err := getLinuxDeviceInfo(name)
			if !opts.match(info) {
				if err != nil && !yield(nil, err) {
					return
				}
				continue
			}

			if !yield(info, err) {
				return
			}
		}
	}
}

func OpenPath(path string) (*Device, error) {
	dev, err := os.OpenFile(path, os.O_RDWR, 0755)
	if err != nil {
		return nil, err
	}
	if err := unix.SetNonblock(int(dev.Fd()), true); err != nil {
		_ = dev.Close()
		return nil, err
	}

	return &Device{
		file: dev,
	}, nil
}

func (d *Device) Read(ctx context.Context, b []byte) (int, error) {
	d.readMu.Lock()
	defer d.readMu.Unlock()

	fd := int(d.file.Fd())
	for {
		if err := waitLinuxReadable(ctx, fd); err != nil {
			return 0, err
		}

		n, err := unix.Read(fd, b)
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		return n, err
	}
}

func (d *Device) Write(ctx context.Context, b []byte) (int, error) {
	buf := append([]byte(nil), b...)
	result := runIO(ctx, &d.writeMu, func() ioResult {
		n, err := d.file.Write(buf)

		return ioResult{n: n, err: err}
	})

	return result.n, result.err
}

func (d *Device) SendFeatureReport(report []byte) error {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		d.file.Fd(),
		hidIOCFeature(0x06, len(report)),
		uintptr(unsafe.Pointer(unsafe.SliceData(report))),
	)

	if errno != 0 {
		return errno
	}

	return nil
}

func (d *Device) GetFeatureReport(report []byte) (int, error) {
	n, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		d.file.Fd(),
		hidIOCFeature(0x07, len(report)),
		uintptr(unsafe.Pointer(unsafe.SliceData(report))),
	)

	if errno != 0 {
		return 0, errno
	}

	return int(n), nil
}

func hidIOCFeature(command, length int) uintptr {
	const (
		iocWrite uintptr = 1
		iocRead  uintptr = 2
	)

	return (iocWrite|iocRead)<<30 |
		uintptr(length)<<16 |
		uintptr('H')<<8 |
		uintptr(command)
}

func (d *Device) Close() error {
	return d.file.Close()
}
