package output

import (
	"fmt"
	"os"
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
	return wrapError("set output directory permissions", path, os.Chmod(path, dirPerm))
}

func chmodFile(path string) error {
	return wrapError("set artifact permissions", path, os.Chmod(path, filePerm))
}
