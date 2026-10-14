//go:build darwin

package jni

/*
#cgo LDFLAGS: -framework Foundation -framework CoreFoundation -framework CoreLocation -framework CoreWLAN
*/
import "C"

func init() {
	// Keeps the cgo preamble live so the desktop dylib links CoreWLAN.
	_ = C.int(0)
}
