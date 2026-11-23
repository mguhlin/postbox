package bridge

import "testing"

func TestDropCapabilityIsScopedSingleUseAndCannotAcceptClientPaths(t *testing.T) {
	b := New(nil, nil)
	first := b.OfferDrop([]string{"/synthetic/one.eml"})
	if _, err := b.takeDrop("/private/arbitrary.eml"); err == nil {
		t.Fatal("path accepted as capability")
	}
	second := b.OfferDrop([]string{"/synthetic/two.eml"})
	if _, err := b.takeDrop(first); err == nil {
		t.Fatal("superseded capability accepted")
	}
	paths, err := b.takeDrop(second)
	if err != nil || len(paths) != 1 || paths[0] != "/synthetic/two.eml" {
		t.Fatal("drop authority changed", err)
	}
	if _, err := b.takeDrop(second); err == nil {
		t.Fatal("reused capability accepted")
	}
}
