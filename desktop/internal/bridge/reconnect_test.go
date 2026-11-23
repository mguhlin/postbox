package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

type fixtureDialogs struct{ root string }

func (d fixtureDialogs) Open(bool) ([]string, error)  { return []string{d.root}, nil }
func (d fixtureDialogs) Save(string) (string, error)  { return "", nil }
func (d fixtureDialogs) Directory() (string, error)   { return d.root, nil }
func (d fixtureDialogs) Preview(string, string) error { return nil }
func (d fixtureDialogs) Print(string) error           { return nil }
func (d fixtureDialogs) ClearPreviews()               {}
func TestReconnectRequiresUniqueUnchangedOriginal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mail.eml")
	os.WriteFile(path, []byte("synthetic original"), 0600)
	hash, _ := fileDigest(path)
	b := New(nil, fixtureDialogs{root})
	changes := []change{{SourceID: "one", Filename: "mail.eml", OldDigest: hash}}
	if err := b.reconnect(changes); err != nil {
		t.Fatal(err)
	}
	if b.sources["one"] != path {
		t.Fatal("original was not reconnected")
	}
	b.sources = map[string]string{}
	os.Mkdir(filepath.Join(root, "copy"), 0700)
	os.WriteFile(filepath.Join(root, "copy/mail.eml"), []byte("synthetic original"), 0600)
	if b.reconnect(changes) == nil || len(b.sources) != 0 {
		t.Fatal("ambiguous match was accepted")
	}
	os.Remove(filepath.Join(root, "copy/mail.eml"))
	os.WriteFile(path, []byte("changed by another app"), 0600)
	if b.reconnect(changes) == nil || len(b.sources) != 0 {
		t.Fatal("changed original was accepted")
	}
}
