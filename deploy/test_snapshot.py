import sqlite3
import tempfile
import unittest
from pathlib import Path
from snapshot import snapshot, validate

class SnapshotTest(unittest.TestCase):
    def test_active_wal_restore(self):
        with tempfile.TemporaryDirectory() as d:
            source = Path(d) / 'source.db'
            backup = Path(d) / 'backup.db'
            db = sqlite3.connect(source)
            db.executescript('PRAGMA journal_mode=WAL; PRAGMA user_version=1;')
            for t in ('users', 'accounts', 'transactions', 'recurring_rules', 'audit'):
                db.execute(f'CREATE TABLE {t}(id INTEGER PRIMARY KEY, amount INTEGER)')
            db.execute('INSERT INTO transactions VALUES(1,31740)'); db.commit()
            manifest = snapshot(source, backup)
            self.assertEqual(manifest['counts']['transactions'], 1)
            with sqlite3.connect(backup) as restored:
                self.assertEqual(restored.execute('SELECT amount FROM transactions').fetchone()[0], 31740)
            db.close()
            self.assertEqual(validate(backup)['schema_version'], 1)
            with self.assertRaises(ValueError): snapshot(source, backup)

if __name__ == '__main__': unittest.main()
