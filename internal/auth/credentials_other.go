//go:build !windows

package auth

// On Unix the 0600 file mode is an explicit per-file at-rest guarantee, so
// protect/unprotect are no-ops. Windows has its own implementation (DPAPI).
func protect(data []byte) ([]byte, error) { return data, nil }

func unprotect(data []byte) ([]byte, error) { return data, nil }
