package bridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type change struct {
	SourceID         string  `json:"sourceId"`
	Filename         string  `json:"filename"`
	OldDigest        string  `json:"oldDigest"`
	Delete           bool    `json:"delete"`
	OriginalWritable bool    `json:"originalWritable"`
	Data             *string `json:"data"`
}

func (b *Bridge) delete(ctx context.Context, p map[string]json.RawMessage) (any, error) {
	originals, err := value[bool](p, "originals")
	if err != nil {
		return nil, err
	}
	var plan struct {
		ID      string   `json:"planId"`
		Changes []change `json:"changes"`
	}
	if err = b.Engine.JSON(ctx, "/plan", p, &plan); err != nil {
		return nil, err
	}
	check := func(c change) error {
		if !c.OriginalWritable {
			return errors.New("PST originals are read-only; MSG attachment edits require an EML/MBOX export or session-only deletion")
		}
		path, ok := b.sources[c.SourceID]
		if !ok {
			return errors.New("Open the original using the desktop Open control before editing it")
		}
		if err := regular(path); err != nil {
			return err
		}
		digest, err := fileDigest(path)
		if err != nil {
			return err
		}
		if digest != c.OldDigest {
			return errors.New("Original changed since import. Reimport it before deleting")
		}
		return nil
	}
	// Preflight the entire batch; a missing or changed source must not partially delete it.
	if originals {
		seen := map[string]bool{}
		for _, c := range plan.Changes {
			if err = check(c); err != nil {
				return nil, err
			}
			path := b.sources[c.SourceID]
			if seen[path] {
				return nil, errors.New("The same original was imported twice. Clear the session and open it once")
			}
			seen[path] = true
		}
	}
	results := []json.RawMessage{}
	failed := func(err error) (any, error) { return map[string]any{"results": results, "error": err.Error()}, nil }
	for _, c := range plan.Changes {
		if originals {
			if err = check(c); err != nil {
				return failed(err)
			}
			path := b.sources[c.SourceID]
			if c.Delete {
				err = os.Remove(path)
			} else if c.Data == nil {
				err = errors.New("Missing replacement content")
			} else {
				var raw []byte
				raw, err = base64.StdEncoding.DecodeString(*c.Data)
				if err == nil {
					err = atomicWrite(path, bytes.NewReader(raw))
				}
			}
			if err != nil {
				return failed(err)
			}
		}
		var result json.RawMessage
		if err = b.Engine.JSON(ctx, "/commit", map[string]string{"planId": plan.ID, "sourceId": c.SourceID}, &result); err != nil {
			if originals {
				err = fmt.Errorf("A disk change completed, but its session update failed. Reimport that source: %w", err)
			}
			return failed(err)
		}
		var update struct {
			Removed  []string        `json:"removedIds"`
			Messages json.RawMessage `json:"messages"`
		}
		json.Unmarshal(result, &update)
		for _, mid := range update.Removed {
			delete(b.messages, mid)
		}
		b.remember(update.Messages, "")
		if !originals || c.Delete {
			delete(b.sources, c.SourceID)
		}
		results = append(results, result)
	}
	return map[string]any{"results": results}, nil
}
