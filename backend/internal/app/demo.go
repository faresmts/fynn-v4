package app

import (
	"database/sql"
	"fmt"
	"time"
)

// SeedDemo is explicitly opt-in and refuses databases containing real users.
func SeedDemo(db *sql.DB) error {
	var count int
	if e := db.QueryRow("SELECT count(*) FROM users").Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		var email string
		if e := db.QueryRow("SELECT email FROM users ORDER BY id LIMIT 1").Scan(&email); e != nil {
			return e
		}
		if email != "alex@demo.local" {
			return fmt.Errorf("não é um banco de demonstração")
		}
		return nil
	}
	if e := Provision(db, "Alex", "alex@demo.local", "demonstracao-local-2026"); e != nil {
		return e
	}
	if e := Provision(db, "Marina", "marina@demo.local", "demonstracao-local-2026"); e != nil {
		return e
	}
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	now, _ := time.Parse("2006-01-02", today())
	start := now.AddDate(0, -6, 0).Format("2006-01") + "-01"
	for _, x := range []struct {
		name, kind string
		amount     int64
	}{{"Nubank", "checking", 350000}, {"Reserva · Inter", "savings", 600000}, {"Carteira", "cash", 25000}} {
		if _, e = tx.Exec("INSERT INTO accounts(household_id,owner_id,name,kind,opening_cents,opening_date) VALUES(1,1,?,?,?,?)", x.name, x.kind, x.amount, start); e != nil {
			return e
		}
	}
	u := User{ID: 1, HouseholdID: 1}
	for i := 5; i >= 0; i-- {
		month := now.AddDate(0, -i, 0).Format("2006-01")
		for j, x := range []struct {
			desc, kind  string
			amount, cat int64
			day         int
		}{{"Salário mensal", "income", 680000, 1, 1}, {"Projeto freelance", "income", 145000, 2, 2}, {"Aluguel do apartamento", "expense", 185000, 5, 2}, {"Supermercado", "expense", 54370, 6, 3}, {"Café & companhia", "expense", 4850, 7, 4}, {"Abastecimento", "expense", 18000, 8, 4}, {"Internet residencial", "expense", 11990, 12, 12}, {"Energia elétrica", "expense", 28000, 5, 18}, {"Streaming", "expense", 3990, 11, 22}} {
			date := fmt.Sprintf("%s-%02d", month, x.day)
			status := "paid"
			var paid *string = &date
			if date > today() {
				status = "pending"
				paid = nil
			}
			visibility := "private"
			if j == 2 || j == 3 || j == 6 || j == 7 {
				visibility = "household"
			}
			amount := x.amount
			if i > 0 && x.kind == "expense" {
				amount += int64(i * 1700)
			}
			_, e = insertTransaction(tx, u, TransactionInput{Type: x.kind, Description: x.desc, Amount: amount, AccountID: 1, CategoryID: &x.cat, Date: date, Competence: month, DueDate: date, Status: status, PaidDate: paid, Visibility: visibility})
			if e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
