//go:build windows
// +build windows

package utils

/*
#cgo CPPFLAGS: -DUNICODE=1
#cgo windows LDFLAGS: -luuid -lole32 -loleaut32
#include <stdlib.h>
int CreateSymLink(const char* path, const char* sympath);
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func CreateLink(path string, lnk string) error {
	cpath := C.CString(path)
	clnk := C.CString(lnk)
	defer C.free(unsafe.Pointer(cpath))
	defer C.free(unsafe.Pointer(clnk))
	hr := C.CreateSymLink(cpath, clnk)
	if hr != 0 {
		return fmt.Errorf("failed to create link: hresult = %d", hr)
	}
	return nil
}
