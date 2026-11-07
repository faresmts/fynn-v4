package store

import (
	"path/filepath"
	"testing"
)

func TestPersistenceAndMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "finance.db")
	db, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO households(id,name) VALUES(1,'Test')"); e != nil {
		t.Fatal(e)
	}
	var mode string
	db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Fatal(mode)
	}
	if _, e = db.Exec("INSERT INTO users(household_id,name,email,password_hash) VALUES(99,'A','a','x')"); e == nil {
		t.Fatal("foreign key not enforced")
	}
	db.Close()
	db, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var n int
	db.QueryRow("SELECT count(*) FROM households").Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	var check string
	db.QueryRow("PRAGMA integrity_check").Scan(&check)
	if check != "ok" {
		t.Fatal(check)
	}
}
