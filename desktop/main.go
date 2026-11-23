package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mguhlin/postbox/desktop/internal/bridge"
	"github.com/mguhlin/postbox/desktop/internal/engine"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed desktop.js desktop-print.js smoke.js postbox.png
var assets embed.FS
var version = "2.0.0"

type dialogs struct {
	app            *application.App
	main           *application.WebviewWindow
	mu             sync.Mutex
	previews       map[string]*application.WebviewWindow
	smoke          string
	onPreviewClose func(string)
}

func (d *dialogs) Open(folder bool) ([]string, error) {
	if d.smoke != "" {
		if folder {
			return []string{filepath.Join(d.smoke, "input")}, nil
		}
		return []string{filepath.Join(d.smoke, "input", "sample.eml")}, nil
	}
	dialog := d.app.Dialog.OpenFile().AttachToWindow(d.main).SetTitle("Open email exports").CanChooseFiles(!folder).CanChooseDirectories(folder)
	if folder {
		path, err := dialog.PromptForSingleSelection()
		if path == "" {
			return nil, err
		}
		return []string{path}, err
	}
	return dialog.AddFilter("Email exports", "*.eml;*.msg;*.pst;*.mbox;*.mbx").PromptForMultipleSelection()
}
func (d *dialogs) Save(name string) (string, error) {
	if d.smoke != "" {
		return filepath.Join(d.smoke, "exports", name), nil
	}
	return d.app.Dialog.SaveFile().AttachToWindow(d.main).SetFilename(name).PromptForSingleSelection()
}
func (d *dialogs) Directory() (string, error) {
	if d.smoke != "" {
		return filepath.Join(d.smoke, "exports"), nil
	}
	return d.app.Dialog.OpenFile().AttachToWindow(d.main).SetTitle("Export attachments here").CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true).PromptForSingleSelection()
}
func (d *dialogs) Preview(id, html string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.previews) >= 10 {
		return errors.New("Close an existing print preview before opening another")
	}
	w := d.app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "Postbox — Print / Save as PDF", Width: 960, Height: 800, URL: "/desktop/preview/" + id})
	d.previews[id] = w
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		d.mu.Lock()
		delete(d.previews, id)
		d.mu.Unlock()
		if d.onPreviewClose != nil {
			d.onPreviewClose(id)
		}
	})
	return nil
}
func (d *dialogs) ClearPreviews() {
	d.mu.Lock()
	windows := d.previews
	d.previews = map[string]*application.WebviewWindow{}
	d.mu.Unlock()
	for _, w := range windows {
		w.Close()
	}
}
func (d *dialogs) Print(id string) error {
	d.mu.Lock()
	w := d.previews[id]
	d.mu.Unlock()
	if w == nil {
		return errors.New("Print preview closed")
	}
	return w.Print()
}
func main() {
	resources := flag.String("resources", "", "application resources directory (development)")
	smoke := flag.String("smoke-test", "", "run synthetic native checks and save a result")
	flag.Parse()
	if *resources == "" {
		binary, err := os.Executable()
		if err != nil {
			panic(err)
		}
		*resources = filepath.Join(filepath.Dir(binary), "resources")
		mac := filepath.Join(filepath.Dir(binary), "..", "Resources")
		if _, err = os.Stat(mac); err == nil {
			*resources = mac
		}
	}
	e, err := engine.Start(*resources)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		showStartupError(err)
		os.Exit(1)
	}
	d := &dialogs{previews: map[string]*application.WebviewWindow{}, smoke: *smoke}
	if *smoke != "" {
		if err = prepareSmoke(*smoke); err != nil {
			e.Close()
			panic(err)
		}
	}
	b := bridge.New(e, d)
	d.onPreviewClose = b.DropPreview
	b.DesktopJS, _ = assets.ReadFile("desktop.js")
	b.PrintJS, _ = assets.ReadFile("desktop-print.js")
	icon, _ := assets.ReadFile("postbox.png")
	d.app = application.New(application.Options{Name: "Postbox", Icon: icon, Description: "Offline email archives — " + version, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true}, Assets: application.AssetOptions{Handler: b}, OnShutdown: e.Close})
	d.main = d.app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "Postbox", Width: 1360, Height: 900, MinWidth: 360, MinHeight: 500, URL: "/"})
	d.main.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { d.ClearPreviews() })
	if *smoke != "" {
		js, _ := assets.ReadFile("smoke.js")
		b.DesktopJS = append(b.DesktopJS, js...)
		b.Smoke = func(raw json.RawMessage) error {
			if err := os.WriteFile(filepath.Join(*smoke, "native-result.json"), raw, 0600); err != nil {
				return err
			}
			go func() { time.Sleep(250 * time.Millisecond); d.app.Quit() }()
			return nil
		}
		go func() {
			time.Sleep(90 * time.Second)
			if _, err := os.Stat(filepath.Join(*smoke, "native-result.json")); err != nil {
				os.WriteFile(filepath.Join(*smoke, "native-result.json"), []byte(`{"ok":false,"error":"Native smoke test timed out"}`), 0600)
				d.app.Quit()
			}
		}()
	}
	if err = d.app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *smoke != "" {
		raw, _ := os.ReadFile(filepath.Join(*smoke, "native-result.json"))
		var result struct {
			OK bool `json:"ok"`
		}
		if json.Unmarshal(raw, &result) != nil || !result.OK {
			os.Exit(1)
		}
	}
}
func samplePDF() []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	content := "BT /F1 18 Tf 40 150 Td (Offline PDF preview) Tj ET"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)}
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 6\n0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return out.Bytes()
}
func prepareSmoke(dir string) error {
	for _, name := range []string{"input", "exports"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "input", "sample.eml"), []byte("From: Ada <ada@example.invalid>\r\nTo: Grace <grace@example.invalid>\r\nSubject: Synthetic native check\r\nDate: Tue, 12 Mar 2024 10:00:00 -0600\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=check\r\n\r\n--check\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<h2>Offline synthetic email</h2><p>Budget approved.</p><script>alert('blocked')</script><img src=\"https://example.invalid/tracker\">\r\n--check\r\nContent-Type: text/plain\r\nContent-Disposition: attachment; filename=proof.txt\r\n\r\nAttachment content\r\n--check\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=proof.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\n"+base64.StdEncoding.EncodeToString(samplePDF())+"\r\n--check--\r\n"), 0600)
}

func showStartupError(err error) {
	app := application.New(application.Options{Name: "Postbox", Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "Postbox — unable to start", Width: 680, Height: 360, HTML: `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'"><title>Postbox startup</title><style>body{margin:2rem;color:#302044;background:#faf7ff;font:1rem/1.6 system-ui,sans-serif}p{overflow-wrap:anywhere}</style></head><body><h1>Postbox could not start</h1><p>` + html.EscapeString(err.Error()) + `</p><p>Extract the complete download, keep the application and resources together, then open Postbox again. Close this window to exit.</p></body></html>`})
	_ = app.Run()
}
