package output

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const reportDirPrefix = "shell-history-report"

func DefaultDirectoryName(t time.Time) string {
	return fmt.Sprintf("%s-%s", reportDirPrefix, t.Local().Format("2006-01-02-150405"))
}

func CreateReportDirectory(cwd string, requested string, now time.Time) (string, error) {
	if requested != "" {
		return createRequestedDirectory(requested)
	}
	return createDefaultDirectory(cwd, now)
}

func createDefaultDirectory(cwd string, now time.Time) (string, error) {
	base := DefaultDirectoryName(now)
	for suffix := 1; ; suffix++ {
		name := base
		if suffix > 1 {
			name = fmt.Sprintf("%s-%d", base, suffix)
		}

		path := filepath.Join(cwd, name)
		if err := os.Mkdir(path, dirPerm); err == nil {
			if err := chmodDir(path); err != nil {
				return "", err
			}
			return path, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", wrapError("create output directory", path, err)
		}
	}
}

func createRequestedDirectory(path string) (string, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.Mode().IsRegular() {
			return "", wrapError("create output directory", path, fmt.Errorf("must not be an existing regular file"))
		}
		if !info.IsDir() {
			return "", wrapError("create output directory", path, fmt.Errorf("must be a directory"))
		}
		return path, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", wrapError("create output directory", path, err)
	}
	if err := os.MkdirAll(path, dirPerm); err != nil {
		return "", wrapError("create output directory", path, err)
	}
	if err := chmodDir(path); err != nil {
		return "", err
	}
	return path, nil
}
