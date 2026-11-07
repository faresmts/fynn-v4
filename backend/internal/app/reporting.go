package app

import (
	"net/http"
	"sort"
	"time"
)

type Totals struct {
	Income  int64 `json:"income_cents"`
	Expense int64 `json:"expense_cents"`
	Pending int64 `json:"pending_cents"`
	Result  int64 `json:"result_cents"`
	Cash    int64 `json:"cash_cents"`
}
type MonthTotal struct {
	Month string `json:"month"`
	Totals
	Count int `json:"count"`
}
type CategoryTotal struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount_cents"`
}
type ContributorTotal struct {
	UserID              int64  `json:"user_id"`
	Name                string `json:"name"`
	PaidCents           int64  `json:"paid_cents"`
	ResponsibilityCents int64  `json:"responsibility_cents"`
	BalanceCents        int64  `json:"balance_cents"`
}
type HouseholdExpense struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
	Status      string `json:"status"`
	DueDate     string `json:"due_date"`
}

func summarize(items []Transaction, month string) Totals {
	var t Totals
	for _, x := range items {
		if x.Status == "cancelled" || x.Type == "transfer" {
			continue
		}
		if x.Competence == month {
			if x.Status == "paid" {
				if x.Type == "income" {
					t.Income += x.Amount
				} else {
					t.Expense += x.Amount
				}
			} else if x.Type == "expense" {
				t.Pending += x.Amount
			}
		}
		if x.Status == "paid" && x.PaidDate != nil && (*x.PaidDate)[:7] == month {
			if x.Type == "income" {
				t.Cash += x.Amount
			} else {
				t.Cash -= x.Amount
			}
		}
	}
	t.Result = t.Income - t.Expense
	return t
}
func (a *App) dashboard(w http.ResponseWriter, r *http.Request, u User) {
	month, ctx, e := period(r)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	items, e := a.transactionList(u, "", ctx)
	if e != nil {
		fail(w, 500, "Não foi possível carregar o resumo.")
		return
	}
	accounts, e := a.accountList(u)
	if e != nil {
		fail(w, 500, "Não foi possível carregar os saldos.")
		return
	}
	var balance *int64
	var available *int64
	if ctx == "personal" {
		var b int64
		for _, x := range accounts {
			b += x.Balance
		}
		balance = &b
		if month == today()[:7] {
			v := b
			for _, x := range items {
				if x.Status == "pending" && x.Type == "expense" && x.DueDate[:7] <= month {
					v -= x.Amount
				}
			}
			available = &v
		}
	} else {
		accounts = []Account{}
	}
	t, _ := time.Parse("2006-01", month)
	history := []MonthTotal{}
	for i := 5; i >= 0; i-- {
		m := t.AddDate(0, -i, 0).Format("2006-01")
		count := 0
		for _, x := range items {
			if x.Competence == m && x.Status == "paid" && x.Type != "transfer" {
				count++
			}
		}
		history = append(history, MonthTotal{Month: m, Totals: summarize(items, m), Count: count})
	}
	cats := map[string]int64{}
	upcoming := []Transaction{}
	recent := []Transaction{}
	for _, x := range items {
		if x.Competence == month {
			if len(recent) < 6 {
				recent = append(recent, x)
			}
			if x.Type == "expense" && x.Status == "paid" {
				cats[x.Category] += x.Amount
			}
		}
		if x.Type == "expense" && x.Status == "pending" && x.DueDate[:7] <= month {
			upcoming = append(upcoming, x)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].DueDate < upcoming[j].DueDate })
	if len(upcoming) > 5 {
		upcoming = upcoming[:5]
	}
	categories := []CategoryTotal{}
	for name, v := range cats {
		categories = append(categories, CategoryTotal{name, v})
	}
	sort.Slice(categories, func(i, j int) bool { return categories[i].Amount > categories[j].Amount })
	contributors := []ContributorTotal{}
	householdExpenses := []HouseholdExpense{}
	if ctx == "household" {
		for _, x := range items {
			if x.Type == "expense" && x.Visibility == "household" && x.Competence == month && x.Status != "cancelled" {
				householdExpenses = append(householdExpenses, HouseholdExpense{ID: x.ID, Description: x.Description, Category: x.Category, AmountCents: x.Amount, Status: x.Status, DueDate: x.DueDate})
			}
		}
		rows, err := a.DB.Query(`SELECT u.id,u.name,
COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.owner_id=u.id AND t.household_id=? AND t.visibility='household' AND t.type='expense' AND t.status='paid' AND t.competence=?),0),
COALESCE((SELECT SUM(s.share_cents) FROM transaction_shares s JOIN transactions t ON t.id=s.transaction_id WHERE s.participant_id=u.id AND t.household_id=? AND t.visibility='household' AND t.type='expense' AND t.status='paid' AND t.competence=?),0)
FROM users u WHERE u.household_id=? ORDER BY u.id`, u.HouseholdID, month, u.HouseholdID, month, u.HouseholdID)
		if err != nil {
			fail(w, 500, "Não foi possível carregar a contribuição de cada morador.")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var c ContributorTotal
			if err = rows.Scan(&c.UserID, &c.Name, &c.PaidCents, &c.ResponsibilityCents); err != nil {
				fail(w, 500, "Falha na leitura das contribuições.")
				return
			}
			c.BalanceCents = c.PaidCents - c.ResponsibilityCents
			contributors = append(contributors, c)
		}
		var totalShared, totalReference int64
		for _, x := range householdExpenses {
			totalShared += x.AmountCents
		}
		for _, c := range contributors {
			totalReference += c.ResponsibilityCents
		}
		if totalReference > 0 {
			var allocated int64
			for i := range contributors {
				if i == len(contributors)-1 {
					contributors[i].ResponsibilityCents = totalShared - allocated
				} else {
					contributors[i].ResponsibilityCents = totalShared * contributors[i].ResponsibilityCents / totalReference
					allocated += contributors[i].ResponsibilityCents
				}
				contributors[i].BalanceCents = contributors[i].PaidCents - contributors[i].ResponsibilityCents
			}
		}
	}
	houseTotals := summarize(items, month)
	if ctx == "household" {
		houseTotals.Income = 0
		houseTotals.Expense = 0
		for _, x := range householdExpenses {
			houseTotals.Expense += x.AmountCents
		}
		houseTotals.Result = -houseTotals.Expense
		houseTotals.Cash = 0
	}
	totals := summarize(items, month)
	if ctx == "household" {
		totals = houseTotals
	}
	reply(w, 200, map[string]any{"month": month, "context": ctx, "totals": totals, "previous": summarize(items, t.AddDate(0, -1, 0).Format("2006-01")), "balance_cents": balance, "available_cents": available, "accounts": accounts, "history": history, "categories": categories, "recent": recent, "upcoming": upcoming, "contributors": contributors, "household_expenses": householdExpenses, "today": today()})
}
func (a *App) settings(w http.ResponseWriter, r *http.Request, u User) {
	var x struct {
		Name            string `json:"name"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decode(w, r, &x) {
		return
	}
	if !textValid(x.Name, 80) {
		fail(w, 400, "Informe um nome de até 80 caracteres.")
		return
	}
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível salvar.")
		return
	}
	defer tx.Rollback()
	if x.NewPassword != "" {
		if len(x.NewPassword) < 12 || len(x.NewPassword) > 1024 || len(x.CurrentPassword) > 1024 {
			fail(w, 400, "Use uma senha de 12 a 1024 caracteres.")
			return
		}
		var hash string
		if e = tx.QueryRow("SELECT password_hash FROM users WHERE id=?", u.ID).Scan(&hash); e != nil || !verifyPassword(x.CurrentPassword, hash) {
			fail(w, 400, "A senha atual está incorreta.")
			return
		}
		if _, e = tx.Exec("UPDATE users SET password_hash=? WHERE id=?", HashPassword(x.NewPassword), u.ID); e == nil {
			_, e = tx.Exec("DELETE FROM sessions WHERE user_id=?", u.ID)
		}
	}
	if e == nil {
		_, e = tx.Exec("UPDATE users SET name=? WHERE id=?", x.Name, u.ID)
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'user',?,'settings')", u.ID, u.ID)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		fail(w, 500, "Não foi possível salvar.")
		return
	}
	reply(w, 200, map[string]bool{"ok": true, "login_required": x.NewPassword != ""})
}
