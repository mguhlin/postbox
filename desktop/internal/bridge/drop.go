package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
)

// OfferDrop is called only by the native OS drop event, never with browser paths.
// The frontend receives a single-use capability, not arbitrary file authority.
func (b *Bridge) OfferDrop(paths []string) string {
	if len(paths) == 0 || len(paths) > 25000 {
		return ""
	}
	nonce := make([]byte, 24)
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dropTicket = hex.EncodeToString(nonce)
	b.dropPaths = append([]string(nil), paths...)
	return b.dropTicket
}

// Called under the bridge lock by the native-action handler.
func (b *Bridge) takeDrop(ticket string) ([]string, error) {
	if ticket == "" || ticket != b.dropTicket {
		return nil, errors.New("This file drop expired. Drop the files again or use Open")
	}
	paths := b.dropPaths
	b.dropTicket = ""
	b.dropPaths = nil
	return paths, nil
}
func (b *Bridge) importPaths(ctx context.Context, paths []string, managed bool) (any, error) {
	results := []json.RawMessage{}
	failures := []string{}
	for _, path := range paths {
		if !supported(path) {
			continue
		}
		duplicate := false
		if managed {
			for _, known := range b.sources {
				if known == path {
					duplicate = true
					break
				}
			}
		}
		if duplicate {
			continue
		}
		raw, err := b.importFile(ctx, path)
		if err != nil {
			failures = append(failures, filepath.Base(path)+": "+err.Error())
			continue
		}
		source := ""
		if managed {
			source = path
		}
		b.remember(raw, source)
		results = append(results, raw)
	}
	return map[string]any{"imports": results, "errors": failures}, nil
}
