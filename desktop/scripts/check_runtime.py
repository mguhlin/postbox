"""Run with the packaged interpreter; no system Python packages are available."""
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]).resolve()))
from outlook import pst_reader
reader = pst_reader()
assert reader.get_version()
import app
message = app.parse_message(b'Subject: Offline runtime check\r\n\r\nSynthetic body', 'check.eml')
assert message['subject'] == 'Offline runtime check'
print('Bundled MIME, MSG/DOC reader and native PST import are available.')
