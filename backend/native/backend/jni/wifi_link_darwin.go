//go:build darwin

package jni

// AppKit stays weak so dlopen of this library in the root daemon does not
// load it. Go rejects -weak_framework unless the darwin dylib recipes set
// CGO_LDFLAGS_ALLOW. This note stays above a blank line so cgo does not
// compile it as C.

/*
#cgo LDFLAGS: -framework Foundation -framework CoreFoundation -framework CoreLocation -framework CoreWLAN -weak_framework AppKit
*/
import "C"

func init() {
	// Keeps the cgo preamble live so the desktop dylib links CoreWLAN.
	_ = C.int(0)
}
