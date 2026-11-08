package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
)

//go:embed schema.sql
var schema string

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+url.PathEscape(path)+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version > 5 {
		db.Close()
		return nil, fmt.Errorf("unsupported schema version %d", version)
	}
	if version == 0 {
		tx, e := db.Begin()
		if e != nil {
			return nil, e
		}
		if _, e = tx.Exec(schema); e != nil {
			tx.Rollback()
			db.Close()
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			return nil, e
		}
		version = 1
	}
	if version == 1 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(`
CREATE TABLE IF NOT EXISTS income_snapshots(
 id INTEGER PRIMARY KEY,
 household_id INTEGER NOT NULL REFERENCES households(id),
 user_id INTEGER NOT NULL REFERENCES users(id),
 month TEXT NOT NULL CHECK(length(month)=7),
 income_cents INTEGER NOT NULL CHECK(income_cents>=0),
 rule_mode TEXT NOT NULL DEFAULT 'monthly_income_ratio' CHECK(rule_mode IN ('monthly_income_ratio','equal','custom')),
 confirmed INTEGER NOT NULL DEFAULT 0,
 version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(household_id,user_id,month)
);
CREATE TABLE IF NOT EXISTS transaction_shares(
 id INTEGER PRIMARY KEY,
 transaction_id INTEGER NOT NULL REFERENCES transactions(id),
 participant_id INTEGER NOT NULL REFERENCES users(id),
 share_cents INTEGER NOT NULL CHECK(share_cents>=0),
 basis TEXT NOT NULL CHECK(basis IN ('monthly_income_ratio','equal','custom')),
 created_at TEXT NOT NULL DEFAULT(strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 UNIQUE(transaction_id,participant_id)
);
CREATE INDEX IF NOT EXISTS income_snapshots_period ON income_snapshots(household_id,month);
CREATE INDEX IF NOT EXISTS transaction_shares_transaction ON transaction_shares(transaction_id);
PRAGMA user_version=2;`)
		if e != nil {
			tx.Rollback()
			db.Close()
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			db.Close()
			return nil, e
		}
		version = 2
	}
	if version == 2 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(`
CREATE TABLE IF NOT EXISTS settlements(
 id INTEGER PRIMARY KEY,
 household_id INTEGER NOT NULL REFERENCES households(id),
 sender_id INTEGER NOT NULL REFERENCES users(id),
 receiver_id INTEGER NOT NULL REFERENCES users(id),
 amount_cents INTEGER NOT NULL CHECK(amount_cents>0),
 month TEXT NOT NULL,
 settled_on TEXT NOT NULL,
 note TEXT NOT NULL DEFAULT '',
 created_by INTEGER NOT NULL REFERENCES users(id),
 UNIQUE(household_id,sender_id,receiver_id,amount_cents,settled_on,note)
);
CREATE TABLE IF NOT EXISTS goals(
 id INTEGER PRIMARY KEY,
 household_id INTEGER NOT NULL REFERENCES households(id),
 owner_id INTEGER NOT NULL REFERENCES users(id),
 name TEXT NOT NULL,
 target_cents INTEGER NOT NULL CHECK(target_cents>0),
 deadline TEXT,
 visibility TEXT NOT NULL DEFAULT 'private' CHECK(visibility IN ('private','household')),
 archived INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS goal_contributions(
 id INTEGER PRIMARY KEY,
 goal_id INTEGER NOT NULL REFERENCES goals(id),
 user_id INTEGER NOT NULL REFERENCES users(id),
 amount_cents INTEGER NOT NULL CHECK(amount_cents>0),
 contributed_on TEXT NOT NULL,
 note TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL DEFAULT(strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS settlements_household_month ON settlements(household_id,month);
CREATE INDEX IF NOT EXISTS goal_contributions_goal ON goal_contributions(goal_id);
PRAGMA user_version=3;`)
		if e != nil {
			tx.Rollback()
			db.Close()
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			db.Close()
			return nil, e
		}
		version = 3
	}
	if version == 3 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(`
CREATE TABLE IF NOT EXISTS card_imports(
 id INTEGER PRIMARY KEY,
 household_id INTEGER NOT NULL REFERENCES households(id),
 owner_id INTEGER NOT NULL REFERENCES users(id),
 institution TEXT NOT NULL CHECK(institution IN ('picpay','nubank')),
 filename TEXT NOT NULL,
 statement_month TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT(strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 confirmed_at TEXT
);
CREATE TABLE IF NOT EXISTS card_import_lines(
 id INTEGER PRIMARY KEY,
 import_id INTEGER NOT NULL REFERENCES card_imports(id) ON DELETE CASCADE,
 purchase_date TEXT NOT NULL,
 description TEXT NOT NULL,
 amount_cents INTEGER NOT NULL CHECK(amount_cents>0),
 card_last4 TEXT,
 installment TEXT,
 transaction_id INTEGER REFERENCES transactions(id),
 UNIQUE(import_id,purchase_date,description,amount_cents)
);
CREATE INDEX IF NOT EXISTS card_import_lines_import ON card_import_lines(import_id);
PRAGMA user_version=4;`)
		if e != nil {
			tx.Rollback()
			db.Close()
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			db.Close()
			return nil, e
		}
		version = 4
	}
	if version == 4 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(`
CREATE TABLE IF NOT EXISTS cards(
 id INTEGER PRIMARY KEY,
 household_id INTEGER NOT NULL REFERENCES households(id), owner_id INTEGER NOT NULL REFERENCES users(id),
 institution TEXT NOT NULL, name TEXT NOT NULL, last4 TEXT, active INTEGER NOT NULL DEFAULT 1,
 UNIQUE(owner_id,institution,name,last4)
);
CREATE TABLE IF NOT EXISTS card_invoices(
 id INTEGER PRIMARY KEY, card_id INTEGER NOT NULL REFERENCES cards(id), reference_month TEXT NOT NULL,
 due_date TEXT NOT NULL, closing_date TEXT, total_cents INTEGER NOT NULL CHECK(total_cents>=0),
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','paid','cancelled')), import_id INTEGER REFERENCES card_imports(id),
 UNIQUE(card_id,reference_month)
);
CREATE TABLE IF NOT EXISTS card_installments(
 id INTEGER PRIMARY KEY, invoice_id INTEGER NOT NULL REFERENCES card_invoices(id) ON DELETE CASCADE,
 description TEXT NOT NULL, purchase_date TEXT NOT NULL, installment_no INTEGER, installment_total INTEGER,
 amount_cents INTEGER NOT NULL CHECK(amount_cents>0), transaction_id INTEGER REFERENCES transactions(id),
 UNIQUE(invoice_id,description,purchase_date,installment_no,amount_cents)
);
CREATE INDEX IF NOT EXISTS card_invoice_period ON card_invoices(card_id,reference_month);
CREATE INDEX IF NOT EXISTS card_installments_invoice ON card_installments(invoice_id);
PRAGMA user_version=5;`)
		if e != nil {
			tx.Rollback()
			db.Close()
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			db.Close()
			return nil, e
		}
	}
	if err = os.Chmod(path, 0600); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
