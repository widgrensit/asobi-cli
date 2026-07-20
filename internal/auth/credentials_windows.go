//go:build windows

package auth

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// On Windows the 0600 file mode only sets the read-only attribute; it creates
// no ACL, so the credential blob would sit in the clear, readable by SYSTEM or
// an Administrator. Encrypt it at rest with DPAPI scoped to the current user
// (CRYPTPROTECT_UI_FORBIDDEN), so a raw read yields ciphertext, not tokens.

var (
	crypt32            = windows.NewLazySystemDLL("crypt32.dll")
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	procCryptProtect   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree      = kernel32.NewProc("LocalFree")
)

const cryptprotectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) dataBlob {
	if len(d) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(d)), pbData: &d[0]}
}

// copyOut copies the DPAPI-allocated output into a Go-owned slice so the
// caller can LocalFree the native buffer.
func (b dataBlob) copyOut() []byte {
	if b.pbData == nil || b.cbData == 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(b.pbData, b.cbData)...)
}

func protect(data []byte) ([]byte, error) {
	in := newBlob(data)
	var out dataBlob
	r, _, err := procCryptProtect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.copyOut(), nil
}

func unprotect(data []byte) ([]byte, error) {
	in := newBlob(data)
	var out dataBlob
	r, _, _ := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		// Not a DPAPI blob: almost certainly a pre-#27 plaintext file (or one
		// written under a different user). Fall back to the raw bytes so an
		// existing login keeps working; the next SaveCredentials re-encrypts.
		return data, nil
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.copyOut(), nil
}
