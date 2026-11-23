# Postbox Desktop components

Postbox Desktop's window, native file controls and application integration are written in Go. The shared mail engine is Python, bundled in each distribution; a Python installation is not needed.

- Wails v3.1.0, MIT: [project](https://github.com/wailsapp/wails/tree/v3.1.0). License included in `licenses/Wails-LICENSE`. Go dependency versions and checksums are in `go.mod` and `go.sum`; the source distribution includes their license notices.
- CPython: included runtime license files. macOS/Linux standalone builds are from [Astral python-build-standalone](https://github.com/astral-sh/python-build-standalone); Windows uses the official Python embeddable distribution.
- libpff Python bindings: LGPL-3.0-or-later; supporting library components retain their original terms. License notices, corresponding source, and provenance are in `resources/app/licenses/` (inside `Postbox.app/Contents/Resources/app/licenses/` on macOS).
- olefile: bundled `resources/app/OLEFILE-LICENSE.txt`.
- Windows WebView2 offline installer: Microsoft software, distributed under Microsoft's runtime terms. Publisher signature is verified during packaging; `webview2-runtime.json` records its version and checksum. The installer runs only if the user needs to install that system runtime.

System WebKit/GTK or Apple's WKWebView are used on Linux/macOS. They are not fetched by Postbox. Public example addresses in checks are fictional.

PDF attachment previews use Mozilla PDF.js (`pdfjs-dist` 6.5.438, Apache-2.0). Its scripts, worker, CMaps and standard fonts are bundled under `resources/app/vendor/pdfjs/`; that directory includes its license, font notices, and package integrity provenance. PDF scripts and embedded actions are not executed by the preview.

The PDF.js compatibility build includes core-js 3.51.0 polyfills (MIT); its notice is retained in `app/vendor/pdfjs/CORE-JS-LICENSE`.
