#!/usr/bin/env python3
"""Create, validate and encrypt a backup into an explicitly configured destination."""
import argparse
import hashlib
import json
import os
import subprocess
import tempfile
from pathlib import Path
from datetime import datetime, timezone
from snapshot import snapshot


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--database', required=True)
    p.add_argument('--destination', required=True, help='Existing mounted external backup directory')
    p.add_argument('--recipient', required=True, help='Public age recipient (not a secret)')
    a = p.parse_args()
    destination = Path(a.destination).resolve()
    if not destination.is_dir():
        p.error('Backup destination must already exist and be mounted')
    os.umask(0o077)
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    output = destination / f'meu-financeiro-{stamp}.db.age'
    with tempfile.TemporaryDirectory(prefix='financeiro-backup-') as tmp:
        plain = Path(tmp) / 'snapshot.db'
        manifest = snapshot(a.database, plain)
        encrypted = Path(tmp) / 'snapshot.db.age'
        subprocess.run(['age', '-r', a.recipient, '-o', str(encrypted), str(plain)], check=True, timeout=120)
        encrypted_hash = hashlib.sha256(encrypted.read_bytes()).hexdigest()
        with output.open('xb') as target, encrypted.open('rb') as source:
            while chunk := source.read(1024 * 1024):
                target.write(chunk)
            target.flush(); os.fsync(target.fileno())
        if hashlib.sha256(output.read_bytes()).hexdigest() != encrypted_hash:
            raise RuntimeError('Encrypted backup checksum mismatch')
        manifest['encrypted_sha256'] = encrypted_hash
        with output.with_suffix('.age.json').open('x') as f:
            json.dump(manifest, f, indent=2)
        print(f'Backup encrypted and verified: {output.name}')


if __name__ == '__main__':
    main()
