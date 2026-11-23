# Postbox — four-guide VCC review

Scope: shared browser viewer, Go desktop integration, native packaging, download page, privacy statement and guides. The review applies [VCC's Web Design](https://mguhlin.github.io/vcc/WEB_DESIGN.md), [SecureVCC](https://mguhlin.github.io/vcc/securevcc/securevcc.md), [Education Privacy](https://mguhlin.github.io/vcc/EDU_COMPLIANCE.md), and [Vibe-Code Rescue](https://mguhlin.github.io/vcc/rescue/VIBE_CODE_RESCUE.md) workflows. This is an engineering review with synthetic data, not a certification or an independent penetration test.

## Web Design

**Function:** open an export, find a message, then read/save/manage it. **Audience:** adults and authorized staff on local devices; geography and institutional approvals are unverified. **Structure:** preserve the existing split archive list/reader, with an immediate Open action and a separate download directory. **Identity:** existing Postbox purple artwork, system body type, serif headings, green Next and red Delete, consistent borders and spacing. **Responsive approach:** one column on narrow screens, content wrapping, scalable labels, visible focus and native dialogs/disclosures. **Target:** WCAG 2.2 AA, not a claim of conformance.

Confirmed accessibility/design issues repaired:

- The root CSP blocked the app's own theme stylesheet. Local stylesheet loading is now allowed while inline executable script is removed.
- Group-selection checkboxes were nested inside interactive disclosure summaries. They now sit outside the summary, retaining group selection and keyboard disclosure.
- Added a skip link, explicit navigation name, larger control/label text, checkbox targets, select/summary focus and narrow-screen wrapping.
- Large lists render 200 additional rows at a time. Search, group selection and Select results still operate on the full matching set.
- PDF previews use bundled PDF.js with extracted page text; rendering is limited to batches and bounded page dimensions. Some encodings need an external reader. Email/PDF content supplied by users can still have accessibility defects.

Evidence: `app/test_browser.cjs` checks browser imports, searches, sandbox network behavior, keyboard skip navigation, dialog Escape, axe, 320-pixel reflow and 200% text. The automated app scan excludes the isolated third-party message frame; it does not establish that imported email is accessible. `app/test_pdf_browser.cjs` checks generated print layout and exports. Native smoke checks include the actual PDF renderer. Physical screen-reader use, a full manual WCAG assessment, all media codecs, and physical printer dialogs remain NOT RUN.

## Security

The public download page is static. The local application handles potentially sensitive archives. There are no remote accounts, tenants, database, payments, AI tools or cloud storage; corresponding multi-tenant controls are N/A. The local file/service boundaries remain relevant.

| Surface / expected rule | Evidence and result | Remaining limit |
| --- | --- | --- |
| Host/origin and session token | PASS: browser request tests reject foreign hosts/origins and missing mutation tokens. Desktop engine additionally requires a private per-launch key. | Browser service is for a trusted device; local OS users/programs are not mutually isolated. |
| Native file authority | PASS: single-use drop capabilities accept only OS-delivered file paths; original paths originate in native Open or explicit reconnect-folder dialogs; saves originate in native destination dialogs. No arbitrary-path bridge action. | Automated native dialogs use synthetic destinations; physical pickers are NOT RUN. |
| Original edits and recovery | PASS: digest preflight, per-source recheck, PST/MSG edit restrictions, atomic replacements, partial-completion reporting, failure preserves existing file. | No undo/automatic backup; same-device concurrent filesystem changes cannot be universally excluded. |
| Export filenames and folders | PASS: safe basenames, reserved-name handling, exclusive creation and symlink checks before descent. | User-chosen destinations and separately opened files retain their own OS access rules. |
| Request/input validation | PASS: object/selection shapes, formats, strict attachment indices, duplicate indices, truncated bodies, MIME nesting and session capacity. Rejection preserves current records. | Limits are not constant-memory guarantees for native parsers or compressed formats. |
| Imported HTML and PDF | PASS: allowlisted HTML, isolated sandbox, blocked navigation/remote content, network-loading CSS rejected, local PDF.js with evaluation disabled and no scripting/action layer. | Complex unsupported mail/PDF/document variants can require another viewer. |
| Reset and stale work | PASS: Clear removes server sources/messages/plans and DOM reader/preview/filter state. A stale preview-close event cannot erase a reopened preview. Desktop print windows close on Clear. | Exports, printouts, OS swap, crash artifacts and independent backups remain. |
| Offline/network behavior | PASS: Linux native smoke runs in a network namespace with only loopback enabled; native HTTP transport only dials its private engine, ignores proxy settings and rejects redirects. Browser synthetic mail causes no external resource requests. | Explicit external links and OS/browser services are outside application processing. Other OSes use CI workflow checks, not an identical network-namespace test. |
| Dependency integrity | PASS for inspected Go build: module checksums and `govulncheck` report no reachable known vulnerabilities. PDF.js package integrity is recorded; native PST source/notices are bundled. Windows runtime signature is checked. | Advisory checks cannot prove absence of unknown vulnerabilities. CPython/libpff/native system libraries need ordinary maintenance. |
| Release identity | Native CI builds/tests the same archived output and emits SHA-256 sidecars. Portable packages and source publish with checksums. | Windows unsigned; Mac ad hoc signed, not notarized. No claimed publisher certificate or independent audit. |

No live credential was confirmed in the reviewed source. The random viewer/engine keys are per-launch local session values, not embedded provider credentials. Review findings were self-reviewed; no independent reviewer is claimed. Effective GitHub account MFA, repository administration policies and host log retention are NOT RUN/unverified rather than inferred from source.

## Education Privacy

**Risk mode B:** local archive processing and deliberate exports, with sensitive content possible. Use with highly sensitive records can require additional institutional review. **Decision:** ready for technical pilot with synthetic data; school-record use remains subject to the institution's authorization. No signed contract, DPA or school approval was supplied or invented.

Data flow: chosen file → local parser/temporary archive → in-memory records and reader/search → user-selected export or confirmed original edit → Clear/close discards app session. Headers, addresses, content, attachments, filenames and folder names can identify people. The privacy statement describes each copy and its clearing limits. GitHub receives normal public website/download request metadata; imported messages are not sent to GitHub. There are no application analytics, advertising, AI, cloud save or remote-message endpoints.

The institution must determine authorized device access, archive retention, export handling and applicable jurisdiction/age rules. [U.S. Department of Education guidance](https://studentprivacy.ed.gov/faq/i-want-use-online-tool-or-application-part-my-course-however-i-am-worried-it-violation-ferpa) supports school review before education-record use. [FTC COPPA guidance](https://www.ftc.gov/business-guidance/resources/complying-coppa-frequently-asked-questions) concerns covered online collection from children; local processing does not establish a blanket exemption or compliance. GDPR/UK/state requirements are UNVERIFIED for any particular institution because its users, records, purpose and jurisdiction are not supplied. No legal deadlines or universal compliance claim are published.

Developer responsibility: retain local defaults, documented controls and accessible error recovery. Institution responsibility: decide authorization, retention, sensitive-record handling and accommodations. User responsibility: protect originals/backups/exports and avoid putting real records in public support issues. Feedback uses the existing GitHub issue channel.

## Vibe-Code Rescue

**Mode:** authorized REPAIR and scoped RELEASE GATE. Baseline: the existing browser/Python implementation and tests were retained; the Go host reuses the same mail engine instead of introducing an independent parser with different behavior.

| Foundation | Result / evidence |
| --- | --- |
| Reproducible setup | Pinned Go modules; packaged Python; architecture-specific Mac PST build; scripts verify the distributed interpreter; native build workflow and guides. |
| Diagnosable failures | Invalid actions return useful errors; startup failures get a native recovery window; native save/import/export cancellation is distinct from success; disk/session partial completion is explicit; no remote logger. |
| Trust boundaries | Host/origin/token checks preserved; isolated mail, private native service, scoped file selections, bounded inputs, no runtime application downloads. |
| Maintainable boundaries | Shared HTML/CSS/JS/Python features; Go engine/process, bridge, native window/dialog and packaging responsibilities are separate. Atomic-write failure and nearby export cases have regression tests. |

Confirmed defects repaired include hidden reader data after reset, blocked theme CSS, malformed selection handling, nested interactive controls, and stale close events clearing a newly opened attachment preview. Existing MIME, MBOX, MSG/PST, document-preview, organizer and print checks are retained. Native drop event wiring is source-reviewed; physical OS drag gestures remain NOT RUN. Native smoke tests exercise single-use drop capabilities, import, search, previews, exports, original edits, token denial, Clear and reimport with real exported bytes. Linux additionally verifies the app without network access. CI results and exact commit identities are available in [native builds](https://github.com/mguhlin/postbox/actions/workflows/desktop.yml); public maintenance dates are intentionally omitted.

**Release gate:** GO for the automated and source-reviewed local workflows, subject to the explicit device/codec/printing/accessibility limits above. This does not approve a school's use of live student records. Reverting source alone does not reverse original-file edits or delete exports; restore those from the user's own backup. Do not replace a release asset without rebuilding, rechecking and regenerating its checksum.
