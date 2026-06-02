package chrome

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// copyDB copies Chrome's Cookies SQLite to a temp file, retrying when
// Chrome holds an exclusive lock (up to ~30s total).
func (r *WindowsReader) copyDB() (string, error) {
	var tmpPath string
	var lastErr error

	for i := 0; i < 10; i++ {
		tmpPath, lastErr = r.copyOnce()
		if lastErr == nil {
			return tmpPath, nil
		}
		time.Sleep(3 * time.Second)
	}

	return "", fmt.Errorf("chrome: copy db after retries: %w", lastErr)
}

// copyOnce attempts a single file copy using Win32 CreateFile with
// FILE_SHARE_READ|FILE_SHARE_WRITE to handle Chrome's WAL lock.
func (r *WindowsReader) copyOnce() (string, error) {
	var tmpID [8]byte
	if _, err := io.ReadFull(rand.Reader, tmpID[:]); err != nil {
		return "", fmt.Errorf("chrome: rand: %w", err)
	}

	tmpPath := filepath.Join(os.TempDir(),
		fmt.Sprintf("cookinc-cookies-%x.db", tmpID[:]))

	src, err := openWithSharedAccess(r.dbPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("chrome: create temp db: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("chrome: copy db: %w", err)
	}

	return tmpPath, nil
}

// openWithSharedAccess opens a file with FILE_SHARE_READ|FILE_SHARE_WRITE,
// allowing reads even when Chrome has the DB locked.
func openWithSharedAccess(path string) (*os.File, error) {
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(
		pathp,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateFile %s: %w", path, err)
	}

	return os.NewFile(uintptr(handle), path), nil
}
