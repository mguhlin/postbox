"""Retain the exact Go module dependency notices in each distribution."""
import json
import shutil
import subprocess
from pathlib import Path
text = subprocess.check_output(['go', 'list', '-m', '-json', 'all'], text=True)
decoder = json.JSONDecoder()
output = Path('build/component-licenses')
output.mkdir(parents=True, exist_ok=True)
while text.strip():
    item, end = decoder.raw_decode(text.lstrip())
    text = text.lstrip()[end:]
    if 'Version' not in item or 'Dir' not in item:
        continue
    source = Path(item['Dir'])
    target = output / item['Path'].replace('/', '_')
    target.mkdir(exist_ok=True)
    for file in source.iterdir():
        if file.is_file() and file.name.upper().startswith(('LICENSE', 'LICENCE', 'COPYING', 'NOTICE', 'COPYRIGHT')):
            shutil.copy2(file, target / file.name)
