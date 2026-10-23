package hid

import (
	"os"
	"sync"
)

type Device struct {
	file    *os.File
	readMu  sync.Mutex
	writeMu sync.Mutex
}

type ioResult struct {
	n   int
	err error
}
