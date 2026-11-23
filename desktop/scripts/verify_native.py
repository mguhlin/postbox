"""Check the native window's result and the actual exported bytes."""
import json
import sys
import zipfile
from pathlib import Path
root = Path(sys.argv[1])
report = json.loads((root / 'native-result.json').read_text())
assert report['ok'], report
assert len(report['checks']) >= 16, report
exports = root / 'exports'
assert (exports / 'proof.txt').read_bytes().strip() == b'Attachment content'
with zipfile.ZipFile(exports / 'Postbox-emails.zip') as archive:
    assert len(archive.namelist()) == 1
    assert b'Synthetic native check' in archive.read(archive.namelist()[0])
assert b'Attachment content' in (exports / 'Postbox-emails.mbox').read_bytes()
assert list(exports.glob('2024/03/*/proof.txt'))
assert not (root / 'input/sample.eml').exists()
print(f"Native application: {len(report['checks'])} checks and exported bytes verified.")
