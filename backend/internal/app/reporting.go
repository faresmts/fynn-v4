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
	Source      string          `json:"source"`
	Version     int             `json:"version"`
	Mode        string          `json:"mode"`
	Shares      map[int64]int64 `json:"shares"`
	OwnerID     int64           `json:"-"`
	ID          int64           `json:"id"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	AmountCents int64           `json:"amount_cents"`
	Status      string          `json:"status"`
	DueDate     string          `json:"due_date"`
}

func (a *App) householdBreakdown(u User, month string) ([]HouseholdExpense, []ContributorTotal, error) {
	tx, e := a.DB.Begin()
	if e != nil {
		return nil, nil, e
	}
	defer tx.Rollback()
	people, e := residents(tx, u, month)
	if e != nil {
		return nil, nil, e
	}
	rows, e := tx.Query(`SELECT h.source,h.id,h.version,h.owner_id,h.description,h.category,h.amount_cents,h.status,h.due_date,
 coalesce(s.participant_id,0),coalesce(s.share_cents,0),coalesce(s.basis,'')
 FROM household_costs h LEFT JOIN household_cost_shares s ON s.source=h.source AND s.id=h.id
 WHERE h.household_id=? AND h.competence=? ORDER BY h.date DESC,h.source,h.id,s.participant_id`, u.HouseholdID, month)
	if e != nil {
		return nil, nil, e
	}
	defer rows.Close()
	costs := []HouseholdExpense{}
	for rows.Next() {
		var x HouseholdExpense
		var uid, cents int64
		if e = rows.Scan(&x.Source, &x.ID, &x.Version, &x.OwnerID, &x.Description, &x.Category, &x.AmountCents, &x.Status, &x.DueDate, &uid, &cents, &x.Mode); e != nil {
			return nil, nil, e
		}
		if len(costs) == 0 || costs[len(costs)-1].Source != x.Source || costs[len(costs)-1].ID != x.ID {
			x.Shares = map[int64]int64{}
			costs = append(costs, x)
		}
		if uid > 0 {
			costs[len(costs)-1].Shares[uid] = cents
		}
	}
	if e = rows.Err(); e != nil {
		return nil, nil, e
	}
	contributors := make([]ContributorTotal, len(people))
	for i, p := range people {
		contributors[i] = ContributorTotal{UserID: p.ID, Name: p.Name}
	}
	for i := range costs {
		x := &costs[i]
		var saved int64
		for _, c := range x.Shares {
			saved += c
		}
		if saved != x.AmountCents {
			x.Mode = "monthly_income_ratio"
			x.Shares, e = allocateSplit(x.AmountCents, people, x.Mode, nil)
			if e != nil {
				x.Mode = "equal"
				x.Shares, e = allocateSplit(x.AmountCents, people, x.Mode, nil)
			}
			if e != nil {
				return nil, nil, e
			}
		}
		for j := range contributors {
			p := &contributors[j]
			p.ResponsibilityCents += x.Shares[p.UserID]
			if p.UserID == x.OwnerID && x.Status == "paid" {
				p.PaidCents += x.AmountCents
			}
			p.BalanceCents = p.PaidCents - p.ResponsibilityCents
		}
	}
	return costs, contributors, nil
}

func summarize(items []Transaction, month string) Totals {
	var t Totals
	for _, x := range items {
		if x.Status == "cancelled" || (x.Type == "transfer" && !x.InvoicePayment) {
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
	if ctx == "household" {
		rows, err := a.DB.Query(`SELECT id,date,description,category,competence,due_date,amount_cents,status FROM household_costs WHERE household_id=? AND source='invoice'`, u.HouseholdID)
		if err != nil {
			fail(w, 500, "Falha ao ler compras compartilhadas.")
			return
		}
		for rows.Next() {
			t := Transaction{Type: "expense", Visibility: "household", Account: "Fatura"}
			if err = rows.Scan(&t.ID, &t.Date, &t.Description, &t.Category, &t.Competence, &t.DueDate, &t.Amount, &t.Status); err != nil {
				break
			}
			items = append(items, t)
		}
		rows.Close()
		if err != nil || rows.Err() != nil {
			fail(w, 500, "Falha ao ler compras compartilhadas.")
			return
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].Date > items[j].Date })
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
			if x.Competence == m && x.Status == "paid" && (x.Type != "transfer" || x.InvoicePayment) {
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
			if (x.Type == "expense" || x.InvoicePayment) && x.Status == "paid" {
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
		householdExpenses, contributors, e = a.householdBreakdown(u, month)
		if e != nil {
			fail(w, 500, "Falha ao consultar os gastos e o rateio da casa.")
			return
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
	cardTotals := []cardMonthTotal{}
	if ctx == "personal" {
		cardTotals, e = a.cardMonthTotals(u, month)
		if e != nil {
			fail(w, 500, "Falha ao consultar as faturas.")
			return
		}
	}
	reply(w, 200, map[string]any{"card_totals": cardTotals, "month": month, "context": ctx, "totals": totals, "previous": summarize(items, t.AddDate(0, -1, 0).Format("2006-01")), "balance_cents": balance, "available_cents": available, "accounts": accounts, "history": history, "categories": categories, "recent": recent, "upcoming": upcoming, "contributors": contributors, "household_expenses": householdExpenses, "today": today()})
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
