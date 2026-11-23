// Package bridge grants native file authority only through OS file dialogs.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mguhlin/postbox/desktop/internal/engine"
)

type Dialogs interface {
	Open(folder bool) ([]string, error)
	Save(name string) (string, error)
	Directory() (string, error)
	Preview(id, html string) error
	Print(id string) error
	ClearPreviews()
}
type Message struct {
	ID          string `json:"id"`
	SourceID    string `json:"sourceId"`
	Subject     string `json:"subject"`
	Date        string `json:"dateKey"`
	Attachments []struct {
		Name string `json:"name"`
	} `json:"attachments"`
}
type Bridge struct {
	Engine    *engine.Engine
	Dialogs   Dialogs
	mu        sync.Mutex
	sources   map[string]string
	messages  map[string]Message
	views     map[string]string
	nextView  int
	DesktopJS []byte
	PrintJS   []byte
	Smoke     func(json.RawMessage) error
}

func New(e *engine.Engine, d Dialogs) *Bridge {
	return &Bridge{Engine: e, Dialogs: d, sources: map[string]string{}, messages: map[string]Message{}, views: map[string]string{}}
}
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == "GET" && r.URL.Path == "/desktop.js" {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(b.DesktopJS)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/desktop-print.js" {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(b.PrintJS)
		return
	}
	if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/desktop/preview/") {
		b.mu.Lock()
		html, ok := b.views[strings.TrimPrefix(r.URL.Path, "/desktop/preview/")]
		b.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, html)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/desktop/") {
		if r.Method != "POST" || r.Header.Get("X-Viewer-Token") != b.Engine.Token {
			http.Error(w, "Native action denied", 403)
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		var p map[string]json.RawMessage
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&p); err != nil || p == nil {
			http.Error(w, "Invalid native request", 400)
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			http.Error(w, "Invalid native request", 400)
			return
		}
		result, err := b.action(r.Context(), r.URL.Path, p)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(result)
		return
	}
	// Wails owns the application origin. Only this proxy knows the engine key.
	headers := r.Header.Clone()
	var body io.Reader = r.Body
	if r.Method == "POST" {
		limit := int64(4 << 20)
		if r.URL.Path == "/import" {
			limit = 100 << 20
		}
		// Webview scheme handlers do not consistently preserve HTTP body length.
		// Buffer bounded copy imports/JSON; native Open streams large PSTs directly.
		raw, readErr := io.ReadAll(io.LimitReader(r.Body, limit+1))
		if readErr != nil || int64(len(raw)) > limit {
			http.Error(w, "Request too large. Use the desktop Open control for PST archives.", 413)
			return
		}
		body = bytes.NewReader(raw)
		r.ContentLength = int64(len(raw))
	}
	if r.ContentLength >= 0 {
		headers.Set("Content-Length", strconv.FormatInt(r.ContentLength, 10))
	}
	res, err := b.Engine.Request(r.Context(), r.Method, r.URL.RequestURI(), body, headers)
	if err != nil {
		http.Error(w, "The local mail engine is unavailable. Close and reopen Postbox.", 502)
		return
	}
	defer res.Body.Close()
	for key, values := range res.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if r.URL.Path == "/" && res.StatusCode == 200 {
		raw, _ := io.ReadAll(res.Body)
		raw = bytes.Replace(raw, []byte(`<script src="/app.js">`), []byte(`<script type="module" src="/wails/runtime.js"></script><script src="/desktop.js"></script><script src="/app.js">`), 1)
		w.Header().Del("Content-Length")
		w.WriteHeader(200)
		w.Write(raw)
		return
	}
	if r.URL.Path == "/import" && res.StatusCode == 200 {
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			http.Error(w, "Import response interrupted", 502)
			return
		}
		b.mu.Lock()
		b.remember(raw, "")
		b.mu.Unlock()
		w.WriteHeader(200)
		w.Write(raw)
		return
	}
	if r.URL.Path == "/clear" && res.StatusCode == 200 {
		b.mu.Lock()
		b.sources = map[string]string{}
		b.messages = map[string]Message{}
		b.views = map[string]string{}
		b.mu.Unlock()
		b.Dialogs.ClearPreviews()
	}
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
}
func value[T any](p map[string]json.RawMessage, key string) (T, error) {
	var v T
	err := json.Unmarshal(p[key], &v)
	return v, err
}
func (b *Bridge) remember(raw json.RawMessage, path string) {
	var list []Message
	if json.Unmarshal(raw, &list) != nil {
		return
	}
	for _, m := range list {
		b.messages[m.ID] = m
		if path != "" {
			b.sources[m.SourceID] = path
		}
	}
}
func (b *Bridge) action(ctx context.Context, path string, p map[string]json.RawMessage) (any, error) {
	switch path {
	case "/desktop/open":
		folder, err := value[bool](p, "folder")
		if err != nil {
			return nil, err
		}
		paths, err := b.Dialogs.Open(folder)
		if err != nil {
			return nil, err
		}
		if folder && len(paths) > 0 {
			root := paths[0]
			paths = nil
			err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && supported(path) {
					paths = append(paths, path)
				}
				if len(paths) > 25000 {
					return errors.New("Choose a folder with fewer than 25,000 email files")
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
		results := []json.RawMessage{}
		failures := []string{}
		for _, path := range paths {
			if !supported(path) {
				continue
			}
			duplicate := false
			for _, known := range b.sources {
				if known == path {
					duplicate = true
					break
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
			b.remember(raw, path)
			results = append(results, raw)
		}
		return map[string]any{"imports": results, "errors": failures}, nil
	case "/desktop/export":
		route, err := value[string](p, "route")
		if err != nil || !(route == "/export" || route == "/export-emails") {
			return nil, errors.New("Invalid export")
		}
		payload := p["payload"]
		var filename string
		if route == "/export" {
			filename = "Postbox-attachments.zip"
		} else {
			filename = "Postbox-emails.zip"
			var format struct {
				Format string `json:"format"`
			}
			json.Unmarshal(payload, &format)
			if format.Format == "mbox" {
				filename = "Postbox-emails.mbox"
			}
		}
		target, err := b.Dialogs.Save(filename)
		if err != nil || target == "" {
			return map[string]bool{"cancelled": true}, err
		}
		res, err := b.Engine.Request(ctx, "POST", route, bytes.NewReader(payload), http.Header{"Content-Type": {"application/json"}})
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if err = checkResponse(res); err != nil {
			return nil, err
		}
		return map[string]bool{"saved": true}, atomicWrite(target, res.Body)
	case "/desktop/save":
		mid, err := value[string](p, "id")
		m, ok := b.messages[mid]
		if err != nil || !ok {
			return nil, errors.New("Reopen this email with the native Open control before saving")
		}
		part, err := value[string](p, "part")
		if err != nil {
			return nil, err
		}
		name := safeName(m.Subject) + ".eml"
		if part != "original" {
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(m.Attachments) {
				return nil, errors.New("Invalid attachment")
			}
			name = safeName(m.Attachments[i].Name)
		}
		target, err := b.Dialogs.Save(name)
		if err != nil || target == "" {
			return map[string]bool{"cancelled": true}, err
		}
		res, err := b.Engine.Request(ctx, "GET", "/"+b.Engine.Token+"/"+mid+"/"+part, nil, nil)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if err = checkResponse(res); err != nil {
			return nil, err
		}
		return map[string]bool{"saved": true}, atomicWrite(target, res.Body)
	case "/desktop/export-folder":
		selection, err := value[map[string][]int](p, "attachments")
		if err != nil || len(selection) == 0 {
			return nil, errors.New("Select attachments first")
		}
		for mid, indices := range selection {
			m, ok := b.messages[mid]
			if !ok {
				return nil, errors.New("Reopen the selected emails with the native Open control")
			}
			for _, i := range indices {
				if i < 0 || i >= len(m.Attachments) {
					return nil, errors.New("Invalid attachment selection")
				}
			}
		}
		root, err := b.Dialogs.Directory()
		if err != nil || root == "" {
			return map[string]bool{"cancelled": true}, err
		}
		saved := 0
		for mid, indices := range selection {
			m := b.messages[mid]
			year, month := "Undated", "Unknown month"
			if len(m.Date) == 10 {
				year, month = m.Date[:4], m.Date[5:7]
			}
			dir, err := exportDirectory(root, year, month, m.Date+"_"+m.Subject+"_"+mid)
			if err != nil {
				return nil, fmt.Errorf("Export stopped after %d attachments: %w", saved, err)
			}
			for _, i := range indices {
				res, err := b.Engine.Request(ctx, "GET", "/"+b.Engine.Token+"/"+mid+"/"+strconv.Itoa(i), nil, nil)
				if err != nil {
					return nil, err
				}
				if err = checkResponse(res); err == nil {
					err = uniqueWrite(dir, m.Attachments[i].Name, res.Body)
				}
				res.Body.Close()
				if err != nil {
					return nil, fmt.Errorf("Export stopped after %d attachments: %w", saved, err)
				}
				saved++
			}
		}
		return map[string]int{"saved": saved}, nil
	case "/desktop/delete":
		return b.delete(ctx, p)
	case "/desktop/preview":
		var html string
		payload := p["payload"]
		res, err := b.Engine.Request(ctx, "POST", "/export-pdf", bytes.NewReader(payload), http.Header{"Content-Type": {"application/json"}})
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if err = checkResponse(res); err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		html = string(raw)
		b.nextView++
		id := strconv.Itoa(b.nextView)
		html = strings.Replace(html, "default-src 'none';", "default-src 'none'; script-src 'self'; connect-src 'self';", 1)
		html = strings.Replace(html, "<title>", `<meta name="viewer-token" content="`+b.Engine.Token+`"><meta name="print-id" content="`+id+`"><script src="/desktop-print.js" defer></script><title>`, 1)
		b.views[id] = html
		if err = b.Dialogs.Preview(id, html); err != nil {
			delete(b.views, id)
			return nil, err
		}
		return map[string]bool{"ready": true}, nil
	case "/desktop/print":
		id, err := value[string](p, "id")
		if err != nil {
			return nil, err
		}
		if _, ok := b.views[id]; !ok {
			return nil, errors.New("Print preview expired")
		}
		return map[string]bool{"printed": true}, b.Dialogs.Print(id)
	case "/desktop/smoke-result":
		if b.Smoke == nil {
			return nil, errors.New("Unavailable")
		}
		return map[string]bool{"ok": true}, b.Smoke(p["result"])
	}
	return nil, errors.New("Unsupported native action")
}
func supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".eml", ".msg", ".pst", ".mbox", ".mbx":
		return true
	}
	return false
}
func checkResponse(res *http.Response) error {
	if res.StatusCode == 200 {
		return nil
	}
	var p struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&p)
	if p.Error == "" {
		p.Error = "The mail engine could not complete this action"
	}
	return errors.New(p.Error)
}
func (b *Bridge) importFile(ctx context.Context, path string) (json.RawMessage, error) {
	if err := regular(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	limit := int64(100 << 20)
	if strings.EqualFold(filepath.Ext(path), ".pst") {
		limit = 2 << 30
	}
	if info.Size() <= 0 || info.Size() > limit {
		return nil, errors.New("PST limit is 2 GB; other imports are limited to 100 MB")
	}
	headers := http.Header{"X-Filename": {url.PathEscape(filepath.Base(path))}}
	res, err := b.Engine.Request(ctx, "POST", "/import", f, headers)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err = checkResponse(res); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(res.Body)
	return json.RawMessage(raw), err
}

func (b *Bridge) DropPreview(id string) { b.mu.Lock(); delete(b.views, id); b.mu.Unlock() }
