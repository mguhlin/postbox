# Postbox — offline email archives

**[Download Postbox](https://mguhlin.github.io/postbox/)** · [Desktop guide](desktop/README.md) · [Browser guide](app/README.md) · [Privacy](privacy.html) · [VCC review](docs/VCC-REVIEW.md)

Created by **Miguel Guhlin** · [SplatWorks.org](https://splatworks.org/) · [Buy Me a Coffee](https://buymeacoffee.com/drawsplat)

Open exported EML, MSG, PST, MBOX and MBX files on your own computer. Search, read, preview, organize, export and delete selected messages or attachments. No account or external mail service is involved.

## Choose an edition

| Edition | Platforms | How it runs |
| --- | --- | --- |
| **Go desktop app** | Windows x64, Linux x64, Apple Silicon and Intel macOS | A native application window with native file dialogs. The shared Python mail engine and local assets are bundled. No separate browser or Python installation. |
| **Portable browser edition** | Windows x64, Linux x64 | Bundled Python starts a local service and opens your web browser. Keep the terminal open; Ctrl+C stops the service. |
| **Source** | Supported desktop platforms | Includes both editions, dependency checksums, build scripts, tests and component notices. |

[Desktop downloads and SHA-256 files](https://github.com/mguhlin/postbox/releases/latest) · [Browser Windows ZIP](downloads/Postbox-windows-x64.zip) · [Browser Linux TAR.GZ](downloads/Postbox-linux-x64.tar.gz) · [Source ZIP](downloads/Postbox-source.zip) · [Browser/source checksums](downloads/SHA256SUMS.txt)

Extract the entire download. Windows desktop uses WebView2; its Microsoft offline installer is included for machines without it. Linux desktop uses system GTK3/WebKitGTK 4.1, which must be installed before offline use. macOS uses the system WKWebView. Windows apps are unsigned and Mac apps are ad hoc signed, not Apple notarized. See the desktop guide for requirements and normal OS approval steps. Postbox does not fetch application code at runtime.

## Shared features

- Open individual messages, whole archives, or folders and subfolders. PST originals stay read-only; Outlook items are converted to MIME for reading/export.
- Read isolated HTML or plain text. Mail scripts, navigation, remote images, and network-loading CSS are removed or blocked.
- Search `from:`, `to:`, `cc:`, `subject:`, `body:`/`content:`, `attachment:`, `has:attachment`, and `has:noattachment`. Combine terms and quote phrases: `from:ada subject:"meeting notes" has:attachment`.
- Group by year/month, filter inclusive dates, select matching results, and Shift-click a checkbox range. Large lists show messages in batches while selection and exports still cover the matching results.
- Preview PDFs locally with bundled PDF.js, images, supported audio/video, text files, DOCX and supported Word 97–2003 DOC text. DOC/DOCX previews omit original page layout and embedded objects. Media codecs depend on the system/browser.
- Save messages and attachments, export selected email as EML ZIP or MBOX, and export attachments to year/month/email folders without overwriting existing exports. Browsers without folder access offer ZIP export.
- Export one or selected emails through a print preview, with Letter/A4 paper and optional clean layout. Each email begins on a new page; long messages continue across pages. Attachment names are listed; attachment contents are not printed.
- Use green **Next** and red **Delete this email** controls. Successful deletion advances through the filtered messages.

## Protect originals and private records

Deletion defaults to **This session only**, which leaves originals intact. Original-file changes require a separate mode choice and confirmation. The desktop Open controls manage local originals on all supported systems; the browser edition requires Chrome/Edge file access. Imported folder copies or dropped files can be browsed without write access. Both editions can reconnect imported copies to a user-selected original folder, checking file contents and rejecting ambiguous matches before editing.

Original-file deletion removes EML/MSG files or rewrites EML/MBOX archives. Attachment deletion changes its source email. PST originals cannot be edited; MSG attachment edits require a session copy or EML/MBOX export. Files are checked against their imported contents before writing. Desktop replacements are atomic. Failures report completed sources; reimport an affected source if a disk change completed before the session update failed. No undo or automatic backup is provided. Keep a separate backup before changing originals.

**Clear session** removes server records, pending deletion plans, reader text, previews, selections and search inputs. Closing the app/service discards the in-memory session. Exports, printouts and source files remain. OS swap, crash remnants, filesystem/browser backups and system webview metadata are outside the app's secure-erasure control. Email can contain personal or school records: keep archives and exports on an authorized device. [Privacy and school-use details](privacy.html).

## Limits

PST imports: 2 GB per file, streamed through a temporary local file by the desktop Open controls and browser service. Other files: 100 MB. Desktop drag-and-drop/copy requests are limited to 100 MB; use native Open for larger PSTs. A session holds at most 25,000 messages and 512 MiB of counted source, message, body and attachment data. Peak parser/process memory can exceed that count. MBOX/PST temporary files are removed after normal parsing; an abrupt process/OS failure can leave temporary artifacts.

Search indexes body text and attachment names, not attached-file contents. CID inline images are offered as attachments. OST/OLM, contacts and calendar-only records are unsupported. Encrypted messages are not decrypted and signatures are not verified. Some PDF fonts/image formats, document variants and media codecs need another compatible viewer. PDF exports are not certified accessible documents.

## Verify and build

```sh
python3 -m unittest discover -s app -p 'test_*.py'
node app/test_search.cjs
cd desktop
go test -race ./internal/...
```

[Desktop build instructions](desktop/README.md) · [Browser build instructions](app/README.md) · [Native CI](https://github.com/mguhlin/postbox/actions/workflows/desktop.yml) · [Portable checks](https://github.com/mguhlin/postbox/actions/workflows/postbox-checks.yml)

`app/test_browser.cjs` covers Chromium, Firefox and WebKit with Playwright and axe; `app/test_pdf_browser.cjs` covers single/batch print exports. Native smoke checks run the real application on each OS with synthetic fixtures and test destinations, then verify actual export bytes. Linux also runs without networking. These are automated checks, not physical-device testing or legal/accessibility certification. The VCC review records scope and remaining gaps.

## Repository and components

[`mguhlin/postbox`](https://github.com/mguhlin/postbox) publishes the static download page from `main` at `/` to **https://mguhlin.github.io/postbox/**. Email processing runs on the user's device; GitHub Pages never runs the mail engine. The existing URL and browser download paths remain stable.

- `app/`: shared browser interface, Python mail engine, local PDF.js assets and tests.
- `desktop/`: Go/Wails application, native file handling, pinned modules and build/package scripts.
- `downloads/`: portable browser packages, complete source archive and checksums.
- `docs/`: implementation and VCC review evidence.

Component license notices and libpff corresponding source are retained. See [desktop third-party notices](desktop/THIRD-PARTY-NOTICES.md), `app/OLEFILE-LICENSE.txt`, `app/licenses/`, and `app/vendor/pdfjs/`. The review follows [VCC's four guides](https://mguhlin.github.io/vcc/).

[Blog introduction: a free way to read Proton Mail and MBOX exports](https://mguhlin.org/blog/2026-11-01-postbox-a-free-way-to-read-your-proton-mail-and-mbox-exports/).
