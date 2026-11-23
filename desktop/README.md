# Postbox Desktop

A Go desktop application for Windows, macOS, and Linux. Read exported email in a native window, without opening a separate browser. The shared Python mail engine and all application assets are bundled. No Python installation, account, or internet connection is needed to use the app.

## Open the app

Extract the entire download and keep its contents together.

- **Windows 10/11 x64:** open `Postbox.exe`. If WebView2 is missing, run the included Microsoft offline runtime installer first. It works without an internet connection. The app is unsigned; Windows may show a publisher warning.
- **macOS 12+ Apple Silicon:** use the arm64 download and open `Postbox.app`.
- **macOS 12+ Intel:** use the x64 download and open `Postbox.app`. Mac packages are ad hoc signed, not Apple notarized; Gatekeeper may require the normal Privacy & Security approval for software downloaded outside the App Store. Organization policy can prohibit unsigned software.
- **Linux x64:** run `./Postbox`. GTK 3 and WebKitGTK 4.1 must already be installed by your operating system. On Debian/Ubuntu their package names are `libgtk-3-0` (or `libgtk-3-0t64`) and `libwebkit2gtk-4.1-0`. These system components may need an administrator's initial setup; Postbox itself never downloads them. Alpine/musl and ARM Linux are not supported by this package.

Closing the Postbox window stops its local mail engine. No launcher terminal needs to stay open. User exports remain on disk.

## Work with an archive

Use **Open email files** or **Open managed folder** for EML, MSG, PST, MBOX and MBX. Search fields, inclusive date filters, grouping, range selection, attachment previews, email and attachment exports, PDF/print preview, Next, and deletion use the same interface and mail engine as the browser edition.

Native save dialogs work on all three operating systems. Attachment exports preserve existing files and use year/month/email folders. **Import folder copy** and drag-and-drop import session copies; the desktop Open controls manage originals immediately. If you later edit an imported copy, choose its original folder when asked; unchanged contents and a unique match are required. Reopening a file already managed in the session skips it.

Deletion defaults to **This session only**. To edit originals, change the deletion mode and confirm. The app verifies source contents before editing, uses atomic replacements, and reports partial completion. There is no undo or automatic backup: keep your own backup before permanently deleting or changing an archive. PST originals are read-only; MSG attachment edits require an EML/MBOX export or a session-only change.

PDF export opens another Postbox window. Choose **Print / Save as PDF** and select the system PDF destination or printer. Letter/A4, single/batch selection, per-email page breaks and clean/original inline formatting are preserved. Printer destinations and media/PDF attachment support depend on the operating system's webview and print services. PDFs list attachments by name, not their contents.

Sessions hold parsed messages in memory; use **Clear session** or close the app to discard them. Limits are 2 GB per PST, 100 MB for other files, 25,000 messages, and 512 MiB of counted session data. Peak parser/OS memory can exceed that counted size. MBOX/PST parsing uses temporary local files deleted after parsing. Device swap, crash recovery, filesystem snapshots, exports and printouts are outside Clear session's erasure scope. See the repository privacy statement for details.

## Build from source

Use Go 1.26+ and the target operating system. Dependency checksums are pinned in `go.sum`. The workflow in `.github/workflows/desktop.yml` builds each target on a matching OS runner, verifies the packaged runtime's PST reader, runs native smoke checks, then creates release archives and SHA-256 files.

```sh
cd desktop
go mod download
go test -race ./internal/...
# Linux: use -tags purego,gtk3,production; GTK3/WebKitGTK 4.1 required at runtime.
CGO_ENABLED=0 go build -tags purego,gtk3,production -o build/Postbox .
python3 scripts/package.py --platform linux-x64 --binary build/Postbox
build/Postbox-Desktop-linux-x64/Postbox --smoke-test /tmp/postbox-native-check
python3 scripts/package.py --platform linux-x64 --archive-only build/Postbox-Desktop-linux-x64
```

macOS packaging additionally needs a CPython 3.13 build interpreter with setuptools and wheel to build the bundled libpff source for the matching architecture. Windows packaging needs the publisher-verified WebView2 offline installer. Build-time downloads are not runtime dependencies. Resource overrides are available with `--resources /absolute/path/to/resources` for development.
