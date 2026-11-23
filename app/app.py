#!/usr/bin/env python3
"""Portable offline email viewer with Outlook import."""
import argparse
import base64
import io
import html as html_module
import json
import mailbox
import secrets
import sys
import tempfile
import threading
import webbrowser
import zipfile
from email import policy
from email.parser import BytesParser
from email.utils import parsedate_to_datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urlparse

ROOT = Path(__file__).parent
sys.path.insert(0, str(ROOT.resolve()))
from previews import MEDIA, preview_kind, text_preview
from organizer import digest, remove_attachments, rebuild_source, safe_filename
from outlook import import_msg, import_pst
from print_export import print_document
from html_safety import safe_css
MESSAGES = {}
SOURCES = {}
PLANS = {}
STATE_LOCK = threading.RLock()
TOKEN = secrets.token_urlsafe(24)
MAX_BYTES = 100 * 1024 * 1024
MAX_PST_BYTES = 2 * 1024 * 1024 * 1024
MAX_MESSAGES = 25000
MAX_SESSION_BYTES = 512 * 1024 * 1024


class SafeEmailHTML(HTMLParser):
    """Keep email formatting while removing navigation and active content."""
    allowed = set('html head body style div span p br hr a img table thead tbody tfoot tr td th col colgroup caption b strong i em u s small big font center blockquote pre code ul ol li dl dt dd h1 h2 h3 h4 h5 h6 sup sub address'.split())
    attributes = set('style class id title dir lang colspan rowspan width height align valign bgcolor color face size cellpadding cellspacing border alt'.split())

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.output = []
        self.suppressed = []
        self.in_style = False

    def handle_starttag(self, tag, attrs):
        if tag in ('script', 'iframe', 'object', 'template'):
            self.suppressed.append(tag)
        if self.suppressed or tag not in self.allowed:
            return
        safe = []
        for key, value in attrs:
            if value is None:
                continue
            if key == 'style':
                value = safe_css(value)
                if not value:
                    continue
            if key in self.attributes or (tag == 'img' and key == 'src' and value.lower().startswith(
                    ('data:image/png;', 'data:image/jpeg;', 'data:image/gif;', 'data:image/webp;', 'data:image/avif;'))):
                safe.append(f' {key}="{html_module.escape(value, quote=True)}"')
        if tag == 'img':
            if not any(item.startswith(' src=') for item in safe):
                alt = next((value for key, value in attrs if key == 'alt' and value), '[Remote or missing image blocked]')
                self.output.append('<span>' + html_module.escape(alt) + '</span>')
                return
            if not any(item.startswith(' alt=') for item in safe):
                safe.append(' alt="Embedded email image"')
        self.output.append('<' + tag + ''.join(safe) + '>')
        self.in_style = tag == 'style' or self.in_style

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        self.handle_endtag(tag)

    def handle_endtag(self, tag):
        if self.suppressed:
            if tag == self.suppressed[-1]:
                self.suppressed.pop()
            return
        if tag in self.allowed and tag not in ('br', 'hr', 'img', 'col'):
            self.output.append('</' + tag + '>')
        if tag == 'style':
            self.in_style = False

    def handle_data(self, data):
        if not self.suppressed:
            self.output.append(safe_css(data) if self.in_style else html_module.escape(data))


def safe_html(content):
    parser = SafeEmailHTML()
    parser.feed(content)
    return ''.join(parser.output)


class BodyText(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.parts = []
        self.hidden = []

    def handle_starttag(self, tag, attrs):
        if tag in ('script', 'style', 'head', 'template'):
            self.hidden.append(tag)
        if not self.hidden and tag in ('br', 'p', 'div', 'li', 'tr', 'h1', 'h2', 'h3'):
            self.parts.append(' ')

    def handle_endtag(self, tag):
        if self.hidden and tag == self.hidden[-1]:
            self.hidden.pop()
        if not self.hidden:
            self.parts.append(' ')

    def handle_data(self, data):
        if not self.hidden:
            self.parts.append(data)


def html_text(content):
    parser = BodyText()
    parser.feed(content)
    return ' '.join(''.join(parser.parts).split())


def parse_export(raw, filename):
    if filename.lower().endswith('.msg'):
        message = parse_message(import_msg(raw), Path(filename).stem + '.eml')
        message['sourceFormat'] = 'msg'
        return [message]
    if filename.lower().endswith('.pst'):
        with tempfile.TemporaryDirectory(prefix='postbox-pst-') as folder:
            path = Path(folder) / 'archive.pst'
            path.write_bytes(raw)
            return import_pst(path, parse_message, filename)
    if not filename.lower().endswith(('.mbox', '.mbx')):
        return [parse_message(raw, filename)]
    if raw.strip() and not raw.startswith(b'From '):
        raise ValueError('Not an MBOX archive')
    # mailbox handles From_ boundaries and nested MIME messages. The temporary
    # archive is deleted immediately after parsing, including on failure.
    with tempfile.TemporaryDirectory(prefix='postbox-') as folder:
        path = Path(folder) / 'archive.mbox'
        path.write_bytes(raw)
        archive = mailbox.mbox(path, create=False)
        try:
            return [parse_message(archive.get_bytes(key, from_=False),
                                  f'{Path(filename).stem}-{i + 1}.eml')
                    for i, key in enumerate(archive.iterkeys())]
        finally:
            archive.close()


def parse_message(raw, filename):
    msg = BytesParser(policy=policy.default).parsebytes(raw)
    attachments = []
    plain, html = [], []
    def leaves(part, depth=0):
        if depth > 40:
            raise ValueError("Email MIME nesting exceeds the supported limit.")
        if part.get_content_type() == 'message/rfc822':
            yield part
        elif part.is_multipart():
            for child in part.iter_parts():
                yield from leaves(child, depth + 1)
        else:
            yield part

    for part in leaves(msg):
        if part.get_content_type() == 'message/rfc822':
            payload = part.get_payload()
            data = b'\n'.join(p.as_bytes() for p in payload) if isinstance(payload, list) else (part.get_payload(decode=True) or b'')
        else:
            data = part.get_payload(decode=True) or b''
        name = part.get_filename()
        if name or part.get_content_disposition() == 'attachment' or part.get_content_maintype() != 'text':
            attachments.append({'name': name or 'attachment', 'data': data,
                                'size': len(data), 'type': part.get_content_type()})
        elif part.get_content_type() in ('text/plain', 'text/html'):
            charset = part.get_content_charset() or 'utf-8'
            try:
                content = data.decode(charset, errors='replace')
            except LookupError:
                content = data.decode('utf-8', errors='replace')
            (html if part.get_content_type() == 'text/html' else plain).append(content)
    date = str(msg.get('Date', ''))
    try:
        date_key = parsedate_to_datetime(date).strftime('%Y-%m-%d')
    except (ValueError, TypeError, OverflowError):
        date_key = ''
    return dict(id=secrets.token_hex(12), filename=filename,
                subject=str(msg.get('Subject', '(No subject)')),
                sender=str(msg.get('From', '')), to=str(msg.get('To', '')),
                cc=str(msg.get('Cc', '')), date=date, dateKey=date_key,
                plain='\n\n'.join(plain), html=safe_html('\n'.join(html)),
                bodyText=' '.join(('\n\n'.join(plain) + ' ' + html_text('\n'.join(html))).split()),
                attachments=attachments, raw=raw)


def public_message(message):
    result = {k: v for k, v in message.items() if k not in ('raw', 'attachments', 'html')}
    result['hasHtml'] = bool(message['html'])
    result['attachments'] = [dict({k: v for k, v in item.items() if k != 'data'},
                                 preview=preview_kind(item)) for item in message['attachments']]
    return result


def validate_selection(payload, attachments=False):
    if not isinstance(payload, dict):
        raise ValueError('Expected a request object.')
    if attachments:
        selection = payload.get('attachments', {})
        if not isinstance(selection, dict) or not selection:
            raise ValueError('Select existing attachments first.')
        for mid, indices in selection.items():
            if mid not in MESSAGES or not isinstance(indices, list) or not indices:
                raise ValueError('Invalid attachment selection.')
            if any(type(i) is not int or not 0 <= i < len(MESSAGES[mid]['attachments']) for i in indices):
                raise ValueError('Invalid attachment selection.')
            if len(set(indices)) != len(indices):
                raise ValueError('Duplicate attachment selection.')
        return selection
    ids = payload.get('messageIds', [])
    if not isinstance(ids, list) or not ids or any(not isinstance(mid, str) or mid not in MESSAGES for mid in ids):
        raise ValueError('Select existing emails first.')
    return list(dict.fromkeys(ids))


def check_session_capacity(imported, raw):
    if len(MESSAGES) + len(imported) > MAX_MESSAGES:
        raise ValueError('Session limit is 25,000 messages. Clear the session or split this archive.')
    def size(message):
        return len(message['raw']) + sum(len(a['data']) for a in message['attachments']) + sum(
            len(message[k].encode('utf-8')) for k in ('plain', 'html', 'bodyText'))
    used = sum(len(source['raw']) for source in SOURCES.values()) + sum(size(m) for m in MESSAGES.values())
    if used + len(raw) + sum(size(m) for m in imported) > MAX_SESSION_BYTES:
        raise ValueError('Session data limit is 512 MiB. Clear the session or split this archive.')


def create_plan(payload):
    action = payload.get('action')
    if action not in ('messages', 'attachments'):
        raise ValueError('Choose messages or attachments to delete.')
    attachment_ids = validate_selection(payload, True) if action == 'attachments' else {}
    ids = set(attachment_ids) if action == 'attachments' else set(validate_selection(payload))
    if not ids or not ids.issubset(MESSAGES):
        raise ValueError('Select existing messages first.')
    source_ids = {MESSAGES[mid]['sourceId'] for mid in ids}
    changes = []
    for sid in source_ids:
        source = SOURCES[sid]
        records = []
        for mid in source['messageIds']:
            message = MESSAGES[mid]
            if action == 'messages' and mid in ids:
                continue
            if action == 'attachments' and mid in ids:
                raw = remove_attachments(message['raw'], set(attachment_ids[mid]))
                updated = parse_message(raw, message['filename'])
                updated.update(id=mid, sourceId=sid, sourceIndex=message['sourceIndex'])
                for field in ('sourceFormat', 'outlookFolder'):
                    if field in message:
                        updated[field] = message[field]
                message = updated
            records.append(message)
        raw = rebuild_source(source, records)
        changes.append(dict(sourceId=sid, filename=source['filename'], oldDigest=digest(source['raw']),
                            originalWritable=not source['filename'].lower().endswith('.pst') and
                                not (source['filename'].lower().endswith('.msg') and action == 'attachments'),
                            raw=raw, records=records, oldIds=list(source['messageIds'])))
    plan_id = secrets.token_hex(16)
    # Keep only the latest plan to avoid retaining abandoned copies of archives.
    PLANS.clear()
    PLANS[plan_id] = changes
    return dict(planId=plan_id, changes=[dict(sourceId=c['sourceId'], filename=c['filename'],
                oldDigest=c['oldDigest'], delete=c['raw'] is None, originalWritable=c['originalWritable'],
                data=base64.b64encode(c['raw']).decode() if c['raw'] is not None and c['originalWritable'] else None) for c in changes])


def commit_plan(payload):
    changes = PLANS.get(payload.get('planId'), [])
    change = next((c for c in changes if c['sourceId'] == payload.get('sourceId')), None)
    if change is None:
        raise ValueError('This change has expired. Select the items again.')
    sid = change['sourceId']
    if sid not in SOURCES or digest(SOURCES[sid]['raw']) != change['oldDigest']:
        raise ValueError('The source has changed. Reimport it before editing.')
    for mid in change['oldIds']:
        MESSAGES.pop(mid, None)
    for index, message in enumerate(change['records']):
        message['sourceIndex'] = index
        MESSAGES[message['id']] = message
    if change['raw'] is None:
        del SOURCES[sid]
    else:
        SOURCES[sid].update(raw=change['raw'], messageIds=[m['id'] for m in change['records']])
    changes.remove(change)
    return dict(removedIds=change['oldIds'], messages=[public_message(m) for m in change['records']])


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def parse_request(self):
        if not super().parse_request():
            return False
        # A loopback bind alone does not establish the browser's origin. Reject
        # rebinding/foreign hostnames before serving the page containing TOKEN.
        desktop_key = getattr(self.server, 'desktop_key', None)
        if desktop_key and self.headers.get('X-Postbox-Desktop') != desktop_key:
            self.send('{"error":"Use the Postbox desktop window."}', status=403)
            return False
        port = self.server.server_port
        authorities = {f'127.0.0.1:{port}', f'localhost:{port}'}
        if port == 80:
            authorities.update(('127.0.0.1', 'localhost'))
        hosts = self.headers.get_all('Host', [])
        host = hosts[0].lower() if len(hosts) == 1 else ''
        origins = self.headers.get_all('Origin', [])
        fetch_site = self.headers.get('Sec-Fetch-Site', '').lower()
        if (host not in authorities or len(origins) > 1
                or (origins and origins[0].lower() != f'http://{host}')
                or fetch_site not in ('', 'none', 'same-origin')):
            self.send('{"error":"Open Postbox using its localhost startup address."}', status=403)
            return False
        return True

    def send(self, data, kind='application/json', status=200, download=None, extra=None):
        if isinstance(data, str):
            data = data.encode('utf-8')
        self.send_response(status)
        self.send_header('Content-Type', kind)
        self.send_header('Content-Length', str(len(data)))
        self.send_header('Cache-Control', 'no-store')
        self.send_header('X-Content-Type-Options', 'nosniff')
        self.send_header('Referrer-Policy', 'no-referrer')
        self.send_header('Permissions-Policy', 'camera=(), microphone=(), geolocation=()')
        if download:
            from urllib.parse import quote
            safe = download.replace('\\', '/').split('/')[-1] or 'attachment'
            self.send_header('Content-Disposition', "attachment; filename*=UTF-8''" + quote(safe, safe=''))
        for key, value in (extra or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        with STATE_LOCK:
            self.handle_get()

    def handle_get(self):
        path = urlparse(self.path).path
        if path in ('/pdf-preview.html', '/pdf-preview.mjs', '/pdf-errors.js') or path.startswith('/vendor/pdfjs/'):
            target = (ROOT / path.lstrip('/')).resolve()
            vendor = (ROOT / 'vendor/pdfjs').resolve()
            if (path.startswith('/vendor/pdfjs/') and (vendor not in target.parents or target.suffix not in ('.mjs', '.bcmap', '.ttf', '.pfb'))):
                self.send('{}', status=404)
                return
            if not target.is_file():
                self.send('{}', status=404)
                return
            kind = 'text/javascript' if target.suffix in ('.js', '.mjs') else 'text/html' if target.suffix == '.html' else 'application/octet-stream'
            self.send(target.read_bytes(), kind)
            return
        if path in ('/about.html', '/theme.css'):
            name = path[1:]
            kind = 'text/html' if name.endswith('.html') else 'text/css'
            self.send((ROOT / name).read_bytes(), kind + '; charset=utf-8')
            return
        if path == '/app.js':
            self.send((ROOT / 'app.js').read_bytes(), 'text/javascript; charset=utf-8')
            return
        if path in ('/hero.png', '/hero-purple.png'):
            self.send((ROOT / path[1:]).read_bytes(), 'image/png')
            return
        if path == '/':
            self.send((ROOT / 'index.html').read_text(encoding='utf-8').replace('__TOKEN__', TOKEN), 'text/html; charset=utf-8', extra={'Content-Security-Policy': "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'; media-src 'self'"})
            return
        pieces = path.strip('/').split('/')
        if len(pieces) >= 3 and pieces[0] == TOKEN and pieces[1] in MESSAGES:
            message = MESSAGES[pieces[1]]
            if pieces[2] == 'preview' and len(pieces) == 4 and pieces[3].isdigit():
                index = int(pieces[3])
                if index >= len(message['attachments']):
                    self.send('{}', status=404)
                    return
                item = message['attachments'][index]
                kind = preview_kind(item)
                if kind == 'text':
                    try:
                        self.send(text_preview(item), 'text/plain; charset=utf-8')
                    except Exception as error:
                        self.send('Preview unavailable. Download this file to open it in a compatible application.\n' + str(error), 'text/plain; charset=utf-8', status=422)
                elif kind:
                    self.send(item['data'], MEDIA[Path(item['name']).suffix.lower()][1], extra={'Content-Security-Policy': "default-src 'none'; frame-ancestors 'self'"})
                else:
                    self.send('No built-in preview. Download this attachment.', 'text/plain', status=415)
                return
            if pieces[2] == 'html':
                self.send(message['html'], 'text/html; charset=utf-8', extra={'Content-Security-Policy': "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'"})
                return
            if pieces[2] == 'original':
                self.send(message['raw'], 'application/octet-stream', download=message['filename'])
                return
            if pieces[2].isdigit() and int(pieces[2]) < len(message['attachments']):
                item = message['attachments'][int(pieces[2])]
                self.send(item['data'], 'application/octet-stream', download=item['name'])
                return
        self.send('{}', status=404)

    def do_POST(self):
        with STATE_LOCK:
            self.handle_post()

    def handle_post(self):
        if self.headers.get('X-Viewer-Token') != TOKEN:
            self.send('{}', status=403)
            return
        if self.headers.get('Transfer-Encoding') or len(self.headers.get_all('Content-Length', [])) != 1:
            self.send('{"error":"A single Content-Length is required."}', status=400)
            return
        if self.path == '/clear':
            MESSAGES.clear()
            SOURCES.clear()
            PLANS.clear()
            self.send('{}')
            return
        if self.path in ('/plan', '/commit', '/export', '/export-emails', '/export-pdf'):
            try:
                length = int(self.headers.get('Content-Length', '0'))
                if not 0 < length < 4 * 1024 * 1024:
                    raise ValueError('Invalid request size.')
                body = self.rfile.read(length)
                if len(body) != length:
                    raise ValueError('The request was interrupted.')
                payload = json.loads(body)
                if not isinstance(payload, dict):
                    raise ValueError('Expected a request object.')
                if self.path == '/export-pdf':
                    ids = validate_selection(payload)
                    if not isinstance(payload.get('clean', True), bool):
                        raise ValueError('Invalid clean layout option.')
                    self.send(print_document([MESSAGES[mid] for mid in dict.fromkeys(ids)],
                                             payload.get('clean', True), payload.get('paper', 'letter')),
                              'text/html; charset=utf-8')
                elif self.path == '/plan':
                    self.send(json.dumps(create_plan(payload)))
                elif self.path == '/commit':
                    self.send(json.dumps(commit_plan(payload)))
                elif self.path == '/export-emails':
                    ids = validate_selection(payload)
                    if payload.get('format') not in ('eml', 'mbox'):
                        raise ValueError('Choose EML ZIP or MBOX export.')
                    if payload.get('format') == 'mbox':
                        raw = rebuild_source(dict(filename='export.pst', raw=b''), [MESSAGES[mid] for mid in ids])
                        self.send(raw, 'application/mbox', download='Postbox-emails.mbox')
                    else:
                        buffer = io.BytesIO()
                        with zipfile.ZipFile(buffer, 'w', zipfile.ZIP_DEFLATED) as archive:
                            for index, mid in enumerate(ids):
                                message = MESSAGES[mid]
                                name = safe_filename(message['subject'])
                                archive.writestr(f'{index + 1:05d}_{message["dateKey"] or "undated"}_{name}.eml', message['raw'])
                        self.send(buffer.getvalue(), 'application/zip', download='Postbox-emails.zip')
                else:
                    buffer = io.BytesIO()
                    with zipfile.ZipFile(buffer, 'w', zipfile.ZIP_DEFLATED) as archive:
                        for mid, indices in validate_selection(payload, True).items():
                            message = MESSAGES[mid]
                            for index in indices:
                                if not isinstance(index, int) or not 0 <= index < len(message['attachments']):
                                    raise ValueError('Invalid attachment selection.')
                                item = message['attachments'][index]
                                archive.writestr(f'{message["dateKey"] or "undated"}_{mid}/{index + 1}_{safe_filename(item["name"])}', item['data'])
                    self.send(buffer.getvalue(), 'application/zip', download='Postbox-attachments.zip')
            except (ValueError, KeyError, TypeError, IndexError) as error:
                self.send(json.dumps({'error': str(error)}), status=400)
            return
        if self.path != '/import':
            self.send('{}', status=404)
            return
        try:
            from urllib.parse import unquote
            filename = unquote(self.headers.get('X-Filename', 'message.eml'))
            filename = filename.replace('\\', '/').split('/')[-1]
            if not filename.lower().endswith(('.eml', '.msg', '.pst', '.mbox', '.mbx')):
                raise ValueError('Choose EML, MSG, PST, MBOX, or MBX files.')
            is_pst = filename.lower().endswith('.pst')
            length = int(self.headers.get('Content-Length', '0'))
            if not 0 < length <= (MAX_PST_BYTES if is_pst else MAX_BYTES):
                self.send(json.dumps({'error': 'PST limit is 2 GB; other imports are limited to 100 MB.'}), status=413)
                return
            if is_pst:
                # Stream large archives to a temporary file instead of keeping
                # an extra multi-gigabyte original in server RAM.
                with tempfile.TemporaryDirectory(prefix='postbox-pst-') as folder:
                    path = Path(folder) / 'archive.pst'
                    with path.open('wb') as output:
                        remaining = length
                        while remaining:
                            chunk = self.rfile.read(min(1024 * 1024, remaining))
                            if not chunk:
                                raise ValueError('The PST upload was interrupted.')
                            output.write(chunk)
                            remaining -= len(chunk)
                    imported = import_pst(path, parse_message, filename)
                raw = b''  # PST originals are never rewritten or retained.
            else:
                raw = self.rfile.read(length)
                if len(raw) != length:
                    raise ValueError('The upload was interrupted. No messages were imported.')
                imported = parse_export(raw, filename)
            check_session_capacity(imported, raw)
            PLANS.clear()
            sid = secrets.token_hex(12)
            SOURCES[sid] = dict(filename=filename, raw=raw, messageIds=[m['id'] for m in imported])
            results = []
            for index, message in enumerate(imported):
                message.update(sourceId=sid, sourceIndex=index)
                MESSAGES[message['id']] = message
                results.append(public_message(message))
            self.send(json.dumps(results))
        except (ValueError, OSError) as error:
            self.send(json.dumps({'error': str(error)}), status=400)
        except Exception:
            self.send('{"error":"This file could not be read. Use a supported email export."}', status=400)


class LocalServer(ThreadingHTTPServer):
    def get_request(self):
        connection, address = super().get_request()
        connection.settimeout(300)
        return connection, address


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--port', type=int, default=0)
    parser.add_argument('--no-open', action='store_true')
    parser.add_argument('--desktop', action='store_true', help=argparse.SUPPRESS)
    args = parser.parse_args()
    server = LocalServer(('127.0.0.1', args.port), Handler)
    server.timeout = 1
    url = f'http://127.0.0.1:{server.server_port}'
    if args.desktop:
        server.desktop_key = sys.stdin.readline().strip()
        if len(server.desktop_key) < 32:
            raise ValueError('Desktop authentication is required.')
        print(json.dumps(dict(url=url, token=TOKEN)), flush=True)
    if not args.desktop:
        print(f'Open {url} — press Ctrl+C to close.', flush=True)
    if not args.no_open and not args.desktop:
        threading.Timer(0.5, lambda: webbrowser.open(url)).start()
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == '__main__':
    main()
