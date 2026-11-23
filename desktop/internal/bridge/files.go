package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeName = regexp.MustCompile(`[\x00-\x1f<>:"/\\|?*]`)
var reserved = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\.|$)`)

func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = unsafeName.ReplaceAllString(name, "_")
	name = strings.Trim(name, " .")
	runes := []rune(name)
	if len(runes) > 150 {
		name = string(runes[:150])
	}
	if name == "" {
		name = "attachment"
	}
	if reserved.MatchString(name) {
		name = "_" + name
	}
	return name
}
func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), err
}
func regular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("Only regular files are supported; symbolic links are not edited")
	}
	return nil
}

// Write replacements next to the destination, sync, then atomically replace it.
// An interrupted write leaves the existing file intact; no hidden backup persists.
func atomicWrite(path string, data io.Reader) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".postbox-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if info, e := os.Stat(path); e == nil {
		if err = f.Chmod(info.Mode().Perm()); err != nil {
			f.Close()
			return err
		}
	}
	if _, err = io.Copy(f, data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// O_EXCL preserves existing exports, including colliding sanitized filenames.
func uniqueWrite(dir, name string, body io.Reader) error {
	name = safeName(name)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; i < 10000; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		path := filepath.Join(dir, candidate)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = io.Copy(f, body)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(path)
		}
		return err
	}
	return errors.New("Too many duplicate filenames")
}
