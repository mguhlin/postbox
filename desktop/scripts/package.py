#!/usr/bin/env python3
"""Package the Go window and shared mail engine with an offline Python runtime."""
import argparse
import hashlib
import json
import os
import plistlib
import shutil
import subprocess
import tarfile
import urllib.request
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DESKTOP = ROOT / 'desktop'
OUT = DESKTOP / 'build'
MAC_RUNTIMES = {
    'macos-arm64': ('aarch64', 'e5af2fde6a937a0e0c795a0c595c4efb70578d57463a2657c819992b9b9acbac'),
    'macos-x64': ('x86_64', '708be9e7df297f68bdae589ea9e62a621db5abe7708f79e851c41410347452d1'),
}


def resources(platform, destination):
    destination.mkdir(parents=True, exist_ok=True)
    app = destination / 'app'
    app.mkdir(exist_ok=True)
    for path in (ROOT / 'app').iterdir():
        if path.is_file() and path.suffix in ('.py', '.js', '.cjs', '.css', '.html', '.md', '.txt', '.png', '.json'):
            shutil.copy2(path, app / path.name)
    shutil.copytree(ROOT / 'app' / 'licenses', app / 'licenses', dirs_exist_ok=True)
    if platform in MAC_RUNTIMES:
        arch, checksum = MAC_RUNTIMES[platform]
        archive = OUT / f'python-{platform}.tar.gz'
        if not archive.exists():
            url = f'https://github.com/astral-sh/python-build-standalone/releases/download/20261026/cpython-3.13.18%2B20261026-{arch}-apple-darwin-install_only.tar.gz'
            urllib.request.urlretrieve(url, archive)
        if hashlib.sha256(archive.read_bytes()).hexdigest() != checksum:
            raise ValueError('Standalone Python checksum mismatch')
        with tarfile.open(archive) as source:
            source.extractall(destination / 'runtime', filter='data')
        # Build the same bundled libpff source for the target Mac architecture.
        # The build runner uses CPython 3.13, matching the bundled interpreter ABI.
        wheel_dir = OUT / 'wheels'
        wheel_dir.mkdir(exist_ok=True)
        subprocess.run([os.environ.get('POSTBOX_BUILD_PYTHON', 'python3'), '-m', 'pip', 'wheel', '--no-deps', '--no-build-isolation',
                        str(ROOT / 'app/licenses/libpff-python-20260926-source.tar.gz'), '-w', str(wheel_dir)], check=True)
        vendor = app / 'vendor' / 'macos'
        vendor.mkdir(parents=True, exist_ok=True)
        for wheel in wheel_dir.glob('*.whl'):
            with zipfile.ZipFile(wheel) as source:
                source.extractall(vendor)
    else:
        # Reuse the verified shipped runtimes, replacing every app source file.
        original = 'Postbox-' + platform
        staging = OUT / 'runtime-source'
        staging.mkdir(parents=True, exist_ok=True)
        if platform == 'windows-x64':
            with zipfile.ZipFile(ROOT / 'downloads' / (original + '.zip')) as source:
                source.extractall(staging)
        else:
            with tarfile.open(ROOT / 'downloads' / (original + '.tar.gz')) as source:
                source.extractall(staging, filter='data')
        shutil.copytree(staging / original / 'runtime', destination / 'runtime', dirs_exist_ok=True)
        vendor = 'windows' if platform.startswith('windows') else 'linux'
        shutil.copytree(ROOT / 'app/vendor' / vendor, app / 'vendor' / vendor, dirs_exist_ok=True)
    # Confirm that the distributed interpreter really imports the native PST reader.
    python = destination / 'runtime' / ('python.exe' if platform.startswith('windows') else 'python/bin/python3')
    subprocess.run([str(python), '-I', str(DESKTOP / 'scripts/check_runtime.py'), str(app)], check=True)
    return app


def package(platform, binary):
    name = 'Postbox-Desktop-' + platform
    folder = OUT / name
    if folder.exists():
        shutil.rmtree(folder)
    folder.mkdir(parents=True)
    if platform.startswith('macos'):
        bundle = folder / 'Postbox.app' / 'Contents'
        (bundle / 'MacOS').mkdir(parents=True)
        shutil.copy2(binary, bundle / 'MacOS/Postbox')
        resource_dir = bundle / 'Resources'
        with (bundle / 'Info.plist').open('wb') as target:
            plistlib.dump({'CFBundleName': 'Postbox', 'CFBundleDisplayName': 'Postbox', 'CFBundleExecutable': 'Postbox',
                          'CFBundleIdentifier': 'org.mguhlin.postbox', 'CFBundlePackageType': 'APPL',
                          'CFBundleShortVersionString': '2.0.0', 'CFBundleVersion': '2.0.0',
                          'LSMinimumSystemVersion': '12.0', 'NSHighResolutionCapable': True}, target)
    else:
        shutil.copy2(binary, folder / ('Postbox.exe' if platform.startswith('windows') else 'Postbox'))
        resource_dir = folder / 'resources'
    resources(platform, resource_dir)
    shutil.copy2(ROOT / 'desktop/README.md', folder / 'START HERE.md')
    if platform.startswith('windows'):
        installer = OUT / 'MicrosoftEdgeWebView2RuntimeInstallerX64.exe'
        if not installer.exists():
            raise ValueError('Provide the publisher-verified Microsoft WebView2 offline installer before packaging Windows')
        shutil.copy2(installer, folder / installer.name)
        shutil.copy2(OUT / 'webview2-runtime.json', folder / 'webview2-runtime.json')
    # Preserve all component licenses and source alongside the runtime.
    shutil.copy2(DESKTOP / 'THIRD-PARTY-NOTICES.md', folder / 'THIRD-PARTY-NOTICES.md')
    shutil.copytree(DESKTOP / 'licenses', folder / 'licenses', dirs_exist_ok=True)
    if (OUT / 'component-licenses').exists():
        shutil.copytree(OUT / 'component-licenses', folder / 'licenses' / 'go-components', dirs_exist_ok=True)
    return folder


def archive(folder):
    release = OUT / 'release'
    release.mkdir(exist_ok=True)
    if folder.name.endswith('linux-x64'):
        output = release / (folder.name + '.tar.gz')
        with tarfile.open(output, 'w:gz', compresslevel=6) as target:
            target.add(folder, arcname=folder.name)
    else:
        output = release / (folder.name + '.zip')
        with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as target:
            for file in folder.rglob('*'):
                if file.is_file():
                    target.write(file, file.relative_to(folder.parent))
    checksum = hashlib.sha256(output.read_bytes()).hexdigest()
    (release / (output.name + '.sha256')).write_text(f'{checksum}  {output.name}\n')
    print(output)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--platform', required=True, choices=['windows-x64', 'linux-x64', *MAC_RUNTIMES])
    parser.add_argument('--binary', type=Path)
    parser.add_argument('--archive-only', type=Path)
    args = parser.parse_args()
    if args.archive_only:
        archive(args.archive_only)
    else:
        folder = package(args.platform, args.binary.resolve())
        print(folder)
