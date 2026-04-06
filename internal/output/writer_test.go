package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteArtifactUsesRestrictivePermissions(t *testing.T) {
	dir := t.TempDir()

	if err := WriteArtifact(dir, ArtifactCheatsheet, []byte("# Shell History Cheat Sheet\n")); err != nil {
		t.Fatalf("WriteArtifact() error = %v", err)
	}

	path := filepath.Join(dir, ArtifactCheatsheet)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "# Shell History Cheat Sheet\n" {
		t.Fatalf("artifact content = %q, want written content", data)
	}
	assertFileMode(t, path, 0o600)
}

func TestWriteArtifactReplacesOnlyKnownArtifact(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(unknown, []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile(unknown) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ArtifactAliasesSH), []byte("old"), 0o600); err != nil {
		t.Fatalf("WriteFile(artifact) error = %v", err)
	}

	if err := WriteArtifact(dir, ArtifactAliasesSH, []byte("new")); err != nil {
		t.Fatalf("WriteArtifact() error = %v", err)
	}

	artifactData, err := os.ReadFile(filepath.Join(dir, ArtifactAliasesSH))
	if err != nil {
		t.Fatalf("ReadFile(artifact) error = %v", err)
	}
	if string(artifactData) != "new" {
		t.Fatalf("artifact content = %q, want replacement", artifactData)
	}

	unknownData, err := os.ReadFile(unknown)
	if err != nil {
		t.Fatalf("ReadFile(unknown) error = %v", err)
	}
	if string(unknownData) != "keep" {
		t.Fatalf("unknown content = %q, want unchanged", unknownData)
	}
}

func TestWriteArtifactRejectsUnknownArtifactNames(t *testing.T) {
	dir := t.TempDir()

	err := WriteArtifact(dir, "unexpected.txt", []byte("nope"))
	if err == nil {
		t.Fatal("WriteArtifact() returned nil error")
	}
	if _, ok := err.(*Error); !ok {
		t.Fatalf("WriteArtifact() error = %T %v, want *Error", err, err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "unexpected.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected artifact exists or stat failed differently: %v", statErr)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%q) = %o, want %o", path, got, want)
	}
}
