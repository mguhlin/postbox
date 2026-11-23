#!/usr/bin/env python3
"""Package repository source, excluding build/runtime output and private Git state."""
import subprocess
import zipfile
from pathlib import Path
root = Path(__file__).resolve().parents[1]
output = root / 'downloads/Postbox-source.zip'
files = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=root).decode().split('\0')
with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as archive:
    for name in sorted(set(files)):
        if not name or name.startswith(('downloads/', '.git/', '.postbox-runtime/', 'desktop/build/')):
            continue
        path = root / name
        if path.is_file():
            archive.write(path, 'Postbox-source/' + name)
print(output)
