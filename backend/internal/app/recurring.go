package app

import (
	"meu-financeiro/internal/domain"
	"net/http"
)

type Rule struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Amount      int64  `json:"amount_cents"`
	Day         int    `json:"day"`
	AccountID   int64  `json:"account_id"`
	CategoryID  int64  `json:"category_id"`
	StartsOn    string `json:"starts_on"`
}

func (a *App) ruleList(u User) ([]Rule, error) {
	rows, e := a.DB.Query("SELECT id,description,amount_cents,day,account_id,category_id,starts_on FROM recurring_rules WHERE owner_id=? AND household_id=? AND active=1 ORDER BY day", u.ID, u.HouseholdID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var x Rule
		if e = rows.Scan(&x.ID, &x.Description, &x.Amount, &x.Day, &x.AccountID, &x.CategoryID, &x.StartsOn); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (a *App) rules(w http.ResponseWriter, r *http.Request, u User) {
	out, e := a.ruleList(u)
	if e != nil {
		fail(w, 500, "Não foi possível carregar as recorrências.")
		return
	}
	reply(w, 200, out)
}
func (a *App) createRule(w http.ResponseWriter, r *http.Request, u User) {
	var x Rule
	if !decode(w, r, &x) {
		return
	}
	if !domain.Month(x.StartsOn) || x.Day < 1 || x.Day > 31 {
		fail(w, 400, "Confira o mês inicial e o dia do vencimento.")
		return
	}
	date, _ := domain.DueDate(x.StartsOn, x.Day)
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível salvar.")
		return
	}
	defer tx.Rollback()
	input := TransactionInput{Type: "expense", Description: x.Description, Amount: x.Amount, AccountID: x.AccountID, CategoryID: &x.CategoryID, Date: date, Competence: x.StartsOn, DueDate: date, Status: "pending", Visibility: "private"}
	if e = validateTransaction(tx, u, input); e != nil {
		fail(w, 400, e.Error())
		return
	}
	res, e := tx.Exec("INSERT INTO recurring_rules(household_id,owner_id,account_id,category_id,description,amount_cents,day,starts_on) VALUES(?,?,?,?,?,?,?,?)", u.HouseholdID, u.ID, x.AccountID, x.CategoryID, x.Description, x.Amount, x.Day, x.StartsOn)
	if e != nil {
		fail(w, 500, "Não foi possível salvar.")
		return
	}
	id, _ := res.LastInsertId()
	if e = tx.Commit(); e != nil {
		fail(w, 500, "Não foi possível salvar.")
		return
	}
	reply(w, 201, map[string]int64{"id": id})
}
func (a *App) generateRules(w http.ResponseWriter, r *http.Request, u User) {
	var input struct {
		Month string `json:"month"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !domain.Month(input.Month) {
		fail(w, 400, "Competência inválida.")
		return
	}
	rules, e := a.ruleList(u)
	if e != nil {
		fail(w, 500, "Não foi possível gerar as contas.")
		return
	}
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível gerar as contas.")
		return
	}
	defer tx.Rollback()
	var count int64
	for _, x := range rules {
		if x.StartsOn > input.Month {
			continue
		}
		date, _ := domain.DueDate(input.Month, x.Day)
		res, err := tx.Exec(`INSERT INTO transactions(household_id,owner_id,type,description,amount_cents,expected_cents,account_id,category_id,date,competence,due_date,status,visibility,recurring_rule_id) VALUES(?,?,'expense',?,?,?,?,?,?,?,?,'pending','private',?) ON CONFLICT(recurring_rule_id,competence) DO NOTHING`, u.HouseholdID, u.ID, x.Description, x.Amount, x.Amount, x.AccountID, x.CategoryID, date, input.Month, date, x.ID)
		if err != nil {
			fail(w, 500, "Não foi possível gerar as contas.")
			return
		}
		n, _ := res.RowsAffected()
		count += n
		if n > 0 {
			id, _ := res.LastInsertId()
			if _, e = tx.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'transaction',?,'recurring')", u.ID, id); e != nil {
				fail(w, 500, "Não foi possível gerar as contas.")
				return
			}
		}
	}
	if e = tx.Commit(); e != nil {
		fail(w, 500, "Não foi possível gerar as contas.")
		return
	}
	reply(w, 200, map[string]int64{"created": count})
}
