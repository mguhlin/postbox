# macOS desktop edition

The Mac edition is implemented as a Go/Wails desktop application for Apple Silicon and Intel Macs, with the shared mail engine and Python runtime bundled. It uses WKWebView and native file dialogs; a separate browser and Python installation are not required.

[Downloads](https://github.com/mguhlin/postbox/releases/latest) · [Desktop instructions and requirements](../desktop/README.md) · [Native build and verification workflow](../.github/workflows/desktop.yml)

Each architecture builds on a matching Mac runner. Packaging compiles the bundled libpff source for that architecture and verifies the distributed interpreter can import it. Synthetic native-window smoke checks verify file operations and exports. The app is ad hoc signed, not Apple notarized. Physical Mac file-picker and system print-dialog checks remain outside automated coverage.
