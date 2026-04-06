package output

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestDefaultDirectoryNameFormat(t *testing.T) {
	now := time.Date(2026, 6, 1, 14, 30, 22, 0, time.Local)

	got := DefaultDirectoryName(now)
	wantPattern := `^shell-history-report-\d{4}-\d{2}-\d{2}-\d{6}$`
	if !regexp.MustCompile(wantPattern).MatchString(got) {
		t.Fatalf("DefaultDirectoryName() = %q, want pattern %s", got, wantPattern)
	}
	if got != "shell-history-report-2026-06-01-143022" {
		t.Fatalf("DefaultDirectoryName() = %q, want timestamped name", got)
	}
}

func TestCreateDefaultDirectoryUsesSuffixes(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 1, 14, 30, 22, 0, time.Local)
	base := filepath.Join(dir, "shell-history-report-2026-06-01-143022")

	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatalf("Mkdir(base) error = %v", err)
	}

	first, err := CreateReportDirectory(dir, "", now)
	if err != nil {
		t.Fatalf("CreateReportDirectory(first) error = %v", err)
	}
	if first != base+"-2" {
		t.Fatalf("CreateReportDirectory(first) = %q, want %q", first, base+"-2")
	}
	assertDirMode(t, first, 0o700)

	if err := os.Mkdir(base+"-2", 0o700); err != nil {
		if !os.IsExist(err) {
			t.Fatalf("Mkdir(base-2) error = %v", err)
		}
	}

	got, err := CreateReportDirectory(dir, "", now)
	if err != nil {
		t.Fatalf("CreateReportDirectory() error = %v", err)
	}
	if got != base+"-3" {
		t.Fatalf("CreateReportDirectory() = %q, want %q", got, base+"-3")
	}
	assertDirMode(t, got, 0o700)
}

func TestCreateRequestedDirectoryRejectsExistingRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report")
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := CreateReportDirectory(dir, path, time.Now())
	if err == nil {
		t.Fatal("CreateReportDirectory() returned nil error")
	}
	if _, ok := err.(*Error); !ok {
		t.Fatalf("CreateReportDirectory() error = %T %v, want *Error", err, err)
	}
}

func TestCreateRequestedDirectoryCreatesWithRestrictivePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom", "nested")

	got, err := CreateReportDirectory(dir, path, time.Now())
	if err != nil {
		t.Fatalf("CreateReportDirectory() error = %v", err)
	}
	if got != path {
		t.Fatalf("CreateReportDirectory() = %q, want %q", got, path)
	}
	assertDirMode(t, got, 0o700)
}

func TestCreateRequestedDirectoryDoesNotDeleteUnknownFiles(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(unknown, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("WriteFile(unknown) error = %v", err)
	}

	got, err := CreateReportDirectory("", dir, time.Now())
	if err != nil {
		t.Fatalf("CreateReportDirectory() error = %v", err)
	}
	if got != dir {
		t.Fatalf("CreateReportDirectory() = %q, want %q", got, dir)
	}

	data, err := os.ReadFile(unknown)
	if err != nil {
		t.Fatalf("ReadFile(unknown) error = %v", err)
	}
	if string(data) != "keep me" {
		t.Fatalf("unknown file content = %q, want preserved", data)
	}
}

func assertDirMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%q) = %o, want %o", path, got, want)
	}
}
