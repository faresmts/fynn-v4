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
	if version > 8 {
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
		version = 5
	}
	if version == 5 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(`ALTER TABLE transactions ADD COLUMN card_id INTEGER REFERENCES cards(id); CREATE INDEX IF NOT EXISTS transactions_card ON transactions(card_id); PRAGMA user_version=6;`)
		if e != nil {
			tx.Rollback()
			db.Close()
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			db.Close()
			return nil, e
		}
		version = 6
	}
	if version == 6 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(`
ALTER TABLE cards ADD COLUMN clearing_account_id INTEGER REFERENCES accounts(id);
ALTER TABLE card_invoices ADD COLUMN payment_transaction_id INTEGER REFERENCES transactions(id);
CREATE UNIQUE INDEX invoice_payment ON card_invoices(payment_transaction_id);
ALTER TABLE card_installments ADD COLUMN category_id INTEGER REFERENCES categories(id);
CREATE TEMP VIEW legacy_card_lines AS
 SELECT t.*, CASE WHEN status='pending' THEN strftime('%Y-%m','now','-3 hours') ELSE competence END AS statement_month
 FROM transactions t WHERE card_id IS NOT NULL AND type='expense' AND status!='cancelled';
INSERT INTO card_invoices(card_id,reference_month,due_date,total_cents)
 SELECT DISTINCT card_id,statement_month,date(statement_month||'-01','+1 month','-1 day'),0
 FROM legacy_card_lines WHERE true ON CONFLICT(card_id,reference_month) DO NOTHING;
INSERT INTO card_installments(invoice_id,description,purchase_date,amount_cents,transaction_id,category_id)
 SELECT i.id,t.description,t.date,t.amount_cents,t.id,t.category_id FROM legacy_card_lines t
 JOIN card_invoices i ON i.card_id=t.card_id AND i.reference_month=t.statement_month
 WHERE NOT EXISTS(SELECT 1 FROM card_installments l WHERE l.transaction_id=t.id);
UPDATE card_invoices SET total_cents=(SELECT sum(amount_cents) FROM card_installments WHERE invoice_id=card_invoices.id),
 status=CASE WHEN EXISTS(SELECT 1 FROM card_installments l LEFT JOIN transactions t ON t.id=l.transaction_id
 WHERE l.invoice_id=card_invoices.id AND coalesce(t.status,'pending')!='paid') THEN 'open' ELSE 'paid' END
 WHERE id IN (SELECT invoice_id FROM card_installments WHERE transaction_id IN (SELECT id FROM legacy_card_lines));
DROP VIEW legacy_card_lines;
PRAGMA user_version=7;`)
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
	if version <= 7 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		_, e = tx.Exec(`
CREATE TABLE shared_invoice_lines (
 line_id INTEGER PRIMARY KEY REFERENCES card_installments(id), version INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE invoice_line_shares (
 line_id INTEGER NOT NULL REFERENCES shared_invoice_lines(line_id),
 participant_id INTEGER NOT NULL REFERENCES users(id), share_cents INTEGER NOT NULL CHECK(share_cents>=0),
 basis TEXT NOT NULL CHECK(basis IN ('monthly_income_ratio','equal','custom')), PRIMARY KEY(line_id,participant_id)
);
CREATE VIEW household_costs AS
 SELECT 'transaction' AS source,t.id,t.household_id,t.owner_id,t.competence,t.date,t.due_date,t.amount_cents,t.description,
 coalesce(k.name,'Despesa') AS category,t.status,t.version
 FROM transactions t LEFT JOIN categories k ON k.id=t.category_id
 WHERE t.card_id IS NULL AND t.type='expense' AND t.visibility='household' AND t.status!='cancelled'
 UNION ALL
 SELECT 'invoice',l.id,c.household_id,c.owner_id,i.reference_month,l.purchase_date,i.due_date,l.amount_cents,l.description,
 'Fatura do cartão',CASE WHEN i.status='paid' OR t.status='paid' THEN 'paid' ELSE 'pending' END,s.version
 FROM shared_invoice_lines s JOIN card_installments l ON l.id=s.line_id JOIN card_invoices i ON i.id=l.invoice_id
 JOIN cards c ON c.id=i.card_id LEFT JOIN transactions t ON t.id=l.transaction_id WHERE i.status!='cancelled';
CREATE VIEW household_cost_shares AS
 SELECT 'transaction' AS source,transaction_id AS id,participant_id,share_cents,basis FROM transaction_shares
 UNION ALL SELECT 'invoice',line_id,participant_id,share_cents,basis FROM invoice_line_shares;
PRAGMA user_version=8;`)
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
