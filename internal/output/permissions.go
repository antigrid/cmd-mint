package output

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const (
	dirPerm  os.FileMode = 0o700
	filePerm os.FileMode = 0o600
)

type Error struct {
	Op   string
	Path string
	Err  error
}

func (e *Error) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("%s %q: %v", e.Op, e.Path, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func wrapError(op string, path string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Op: op, Path: path, Err: err}
}

func chmodDir(path string) error {
	return chmodBestEffort("set output directory permissions", path, dirPerm)
}

func chmodFile(path string) error {
	return chmodBestEffort("set artifact permissions", path, filePerm)
}

func chmodBestEffort(op string, path string, perm os.FileMode) error {
	err := os.Chmod(path, perm)
	if err == nil || permissionChangeUnsupported(err) {
		return nil
	}
	return wrapError(op, path, err)
}

func permissionChangeUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP) ||
		errors.Is(err, syscall.ENOSYS) ||
		errors.Is(err, syscall.EPERM)
}
