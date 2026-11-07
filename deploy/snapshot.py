#!/usr/bin/env python3
"""Consistent SQLite snapshot using the online backup API; no raw WAL copying."""
import argparse
import hashlib
import json
import os
import sqlite3
import tempfile
from pathlib import Path
from datetime import datetime, timezone


def validate(path):
    with sqlite3.connect(f'{Path(path).resolve().as_uri()}?mode=ro', uri=True) as db:
        if db.execute('PRAGMA integrity_check').fetchall() != [('ok',)]:
            raise RuntimeError('SQLite integrity check failed')
        if db.execute('PRAGMA foreign_key_check').fetchall():
            raise RuntimeError('SQLite foreign key check failed')
        version = db.execute('PRAGMA user_version').fetchone()[0]
        counts = {}
        for table in ('users', 'accounts', 'transactions', 'recurring_rules', 'audit'):
            counts[table] = db.execute(f'SELECT count(*) FROM {table}').fetchone()[0]
        return {'schema_version': version, 'counts': counts}


def snapshot(source, destination):
    source, destination = Path(source).resolve(), Path(destination).resolve()
    if source == destination or destination.exists():
        raise ValueError('Destination must be a new file different from the source')
    destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, temp = tempfile.mkstemp(prefix='.snapshot-', dir=destination.parent)
    os.close(fd)
    try:
        with sqlite3.connect(f'{source.as_uri()}?mode=ro', uri=True) as src:
            with sqlite3.connect(temp) as dst:
                src.backup(dst)
        manifest = validate(temp)
        manifest['created_at'] = datetime.now(timezone.utc).isoformat()
        manifest['sha256'] = hashlib.sha256(Path(temp).read_bytes()).hexdigest()
        os.link(temp, destination)  # Fails if another writer created the destination.
        return manifest
    finally:
        Path(temp).unlink(missing_ok=True)


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('source'); p.add_argument('destination')
    args = p.parse_args()
    print(json.dumps(snapshot(args.source, args.destination), indent=2))
