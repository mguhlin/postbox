package bridge

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("synthetic read failure") }
func TestAtomicFailurePreservesOriginalAndRemovesTemporaryCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.eml")
	os.WriteFile(path, []byte("original"), 0600)
	if atomicWrite(path, brokenReader{}) == nil {
		t.Fatal("expected failure")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "original" {
		t.Fatal("original changed")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("temporary copy retained")
	}
	if err := atomicWrite(path, bytes.NewReader([]byte("replacement"))); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if string(raw) != "replacement" {
		t.Fatal("replacement missing")
	}
}
func TestExportNeverOverwritesAndSanitizesNames(t *testing.T) {
	dir := t.TempDir()
	for _, content := range []string{"one", "two"} {
		if err := uniqueWrite(dir, "../../CON.txt", bytes.NewBufferString(content)); err != nil {
			t.Fatal(err)
		}
	}
	a, _ := os.ReadFile(filepath.Join(dir, "_CON.txt"))
	b, _ := os.ReadFile(filepath.Join(dir, "_CON (1).txt"))
	if string(a) != "one" || string(b) != "two" {
		t.Fatal("existing export overwritten")
	}
	if err := uniqueWrite(dir, "bad.txt", brokenReader{}); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "bad.txt")); !os.IsNotExist(err) {
		t.Fatal("partial export retained")
	}
	_ = io.EOF
}
func TestSymlinkIsNotManagedOriginal(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("symlink permission depends on developer mode")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "original")
	os.WriteFile(path, []byte("original"), 0600)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Skip(err)
	}
	if regular(link) == nil {
		t.Fatal("symbolic link accepted")
	}
}

func TestExportRejectsSymlinkBeforeCreatingOutsideDirectory(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "2024")); err != nil {
		t.Skip(err)
	}
	if _, err := exportDirectory(root, "2024", "03", "mail"); err == nil {
		t.Fatal("symlink export accepted")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("created directories outside selected root")
	}
}
