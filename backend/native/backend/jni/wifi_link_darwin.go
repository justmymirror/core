//go:build darwin

package jni

/*
#cgo LDFLAGS: -framework Foundation -framework CoreFoundation -framework CoreLocation -framework CoreWLAN -weak_framework AppKit
*/
import "C"

func init() {
	// Keeps the cgo preamble live so the desktop dylib links CoreWLAN.
	_ = C.int(0)
}
