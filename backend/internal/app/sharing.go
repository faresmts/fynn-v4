package app

import (
	"fmt"
	"meu-financeiro/internal/domain"
	"net/http"
	"strconv"
)

type IncomeSnapshot struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	Month       string `json:"month"`
	IncomeCents int64  `json:"income_cents"`
	RuleMode    string `json:"rule_mode"`
	Confirmed   bool   `json:"confirmed"`
	Version     int    `json:"version"`
}

func (a *App) incomeSnapshots(w http.ResponseWriter, r *http.Request, u User) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = today()[:7]
	}
	if !domain.Month(month) {
		fail(w, 400, "Competência inválida.")
		return
	}
	rows, e := a.DB.Query(`SELECT s.id,s.user_id,u.name,s.month,s.income_cents,s.rule_mode,s.confirmed,s.version
FROM income_snapshots s JOIN users u ON u.id=s.user_id
WHERE s.household_id=? AND s.month=? ORDER BY s.user_id`, u.HouseholdID, month)
	if e != nil {
		fail(w, 500, "Não foi possível carregar as rendas de referência.")
		return
	}
	defer rows.Close()
	out := []IncomeSnapshot{}
	for rows.Next() {
		var x IncomeSnapshot
		var confirmed int
		if e = rows.Scan(&x.ID, &x.UserID, &x.UserName, &x.Month, &x.IncomeCents, &x.RuleMode, &confirmed, &x.Version); e != nil {
			fail(w, 500, "Falha na leitura.")
			return
		}
		x.Confirmed = confirmed == 1
		out = append(out, x)
	}
	reply(w, 200, out)
}

func (a *App) saveIncomeSnapshot(w http.ResponseWriter, r *http.Request, u User) {
	var x struct {
		UserID      int64  `json:"user_id"`
		Month       string `json:"month"`
		IncomeCents int64  `json:"income_cents"`
		RuleMode    string `json:"rule_mode"`
		Confirmed   bool   `json:"confirmed"`
		Version     int    `json:"version"`
	}
	if !decode(w, r, &x) {
		return
	}
	if x.UserID != u.ID {
		var count int
		if e := a.DB.QueryRow("SELECT count(*) FROM users WHERE id=? AND household_id=?", x.UserID, u.HouseholdID).Scan(&count); e != nil || count != 1 {
			fail(w, 404, "Pessoa não encontrada na família.")
			return
		}
	}
	if !domain.Month(x.Month) || x.IncomeCents < 0 || x.IncomeCents > domain.MaxCents {
		fail(w, 400, "Confira a competência e a renda líquida de referência.")
		return
	}
	if x.RuleMode == "" {
		x.RuleMode = "monthly_income_ratio"
	}
	if x.RuleMode != "monthly_income_ratio" && x.RuleMode != "equal" && x.RuleMode != "custom" {
		fail(w, 400, "Regra de rateio inválida.")
		return
	}
	result, e := a.DB.Exec(`INSERT INTO income_snapshots(household_id,user_id,month,income_cents,rule_mode,confirmed)
VALUES(?,?,?,?,?,?) ON CONFLICT(household_id,user_id,month) DO UPDATE SET income_cents=excluded.income_cents,rule_mode=excluded.rule_mode,confirmed=excluded.confirmed,version=income_snapshots.version+1`, u.HouseholdID, x.UserID, x.Month, x.IncomeCents, x.RuleMode, boolInt(x.Confirmed))
	if e != nil {
		fail(w, 500, "Não foi possível salvar a renda de referência.")
		return
	}
	id, _ := result.LastInsertId()
	reply(w, 200, map[string]any{"ok": true, "id": id})
}

func (a *App) sharingSummary(w http.ResponseWriter, r *http.Request, u User) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = today()[:7]
	}
	if !domain.Month(month) {
		fail(w, 400, "Competência inválida.")
		return
	}
	rows, e := a.DB.Query(`SELECT u.id,u.name,COALESCE(s.income_cents,0),COALESCE(s.confirmed,0)
FROM users u LEFT JOIN income_snapshots s ON s.user_id=u.id AND s.household_id=u.household_id AND s.month=? WHERE u.household_id=? ORDER BY u.id`, month, u.HouseholdID)
	if e != nil {
		fail(w, 500, "Não foi possível carregar o resumo da casa.")
		return
	}
	defer rows.Close()
	type person struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		IncomeCents int64  `json:"income_cents"`
		Confirmed   bool   `json:"confirmed"`
		RatioBps    int64  `json:"ratio_bps"`
	}
	people := []person{}
	var total int64
	for rows.Next() {
		var p person
		var c int
		if e = rows.Scan(&p.ID, &p.Name, &p.IncomeCents, &c); e != nil {
			fail(w, 500, "Falha na leitura.")
			return
		}
		p.Confirmed = c == 1
		total += p.IncomeCents
		people = append(people, p)
	}
	for i := range people {
		if total > 0 {
			people[i].RatioBps = people[i].IncomeCents * 10000 / total
		}
	}
	if total == 0 {
		reply(w, 200, map[string]any{"month": month, "status": "needs_rule", "people": people, "total_income_cents": int64(0), "message": "Confirme as rendas ou escolha divisão manual antes de ratear."})
		return
	}
	reply(w, 200, map[string]any{"month": month, "status": "ready", "people": people, "total_income_cents": total})
}

func (a *App) splitTransaction(w http.ResponseWriter, r *http.Request, u User) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		fail(w, 404, "Despesa não encontrada.")
		return
	}
	var x struct {
		Version int              `json:"version"`
		Mode    string           `json:"mode"`
		Custom  map[string]int64 `json:"custom_cents"`
	}
	if !decode(w, r, &x) {
		return
	}
	if x.Mode != "monthly_income_ratio" && x.Mode != "equal" && x.Mode != "custom" {
		fail(w, 400, "Regra de rateio inválida.")
		return
	}
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível ratear a despesa.")
		return
	}
	defer tx.Rollback()
	var amount, version int
	var month string
	var owner int64
	e = tx.QueryRow(`SELECT amount_cents,version,competence,owner_id FROM transactions WHERE id=? AND household_id=? AND type='expense' AND visibility='household' AND status!='cancelled'`, id, u.HouseholdID).Scan(&amount, &version, &month, &owner)
	if e != nil {
		fail(w, 404, "Despesa compartilhada não encontrada.")
		return
	}
	if version != x.Version {
		fail(w, 409, "Esta despesa mudou. Atualize a página e tente novamente.")
		return
	}
	var users []int64
	rows, e := tx.Query("SELECT id FROM users WHERE household_id=? ORDER BY id", u.HouseholdID)
	if e != nil {
		fail(w, 500, "Não foi possível ratear a despesa.")
		return
	}
	for rows.Next() {
		var uid int64
		rows.Scan(&uid)
		users = append(users, uid)
	}
	rows.Close()
	if len(users) != 2 {
		fail(w, 400, "O rateio desta versão exige duas pessoas na família.")
		return
	}
	shares := map[int64]int64{}
	switch x.Mode {
	case "equal":
		shares[users[0]] = int64(amount / 2)
		shares[users[1]] = int64(amount) - shares[users[0]]
	case "custom":
		var total int64
		for _, uid := range users {
			v, ok := x.Custom[strconv.FormatInt(uid, 10)]
			if !ok || v < 0 {
				fail(w, 400, "Informe as cotas das duas pessoas.")
				return
			}
			shares[uid] = v
			total += v
		}
		if total != int64(amount) {
			fail(w, 400, "As cotas devem somar exatamente o valor da despesa.")
			return
		}
	case "monthly_income_ratio":
		var i1, i2 int64
		if e = tx.QueryRow("SELECT COALESCE(income_cents,0) FROM income_snapshots WHERE household_id=? AND user_id=? AND month=? AND confirmed=1", u.HouseholdID, users[0], month).Scan(&i1); e != nil {
			fail(w, 400, "Confirme as duas rendas de referência antes de usar o rateio proporcional.")
			return
		}
		if e = tx.QueryRow("SELECT COALESCE(income_cents,0) FROM income_snapshots WHERE household_id=? AND user_id=? AND month=? AND confirmed=1", u.HouseholdID, users[1], month).Scan(&i2); e != nil || i1+i2 <= 0 {
			fail(w, 400, "Confirme as duas rendas de referência antes de usar o rateio proporcional.")
			return
		}
		shares[users[0]] = int64(amount) * i1 / (i1 + i2)
		shares[users[1]] = int64(amount) - shares[users[0]]
	}
	if _, e = tx.Exec("DELETE FROM transaction_shares WHERE transaction_id=?", id); e == nil {
		for _, uid := range users {
			if _, e = tx.Exec("INSERT INTO transaction_shares(transaction_id,participant_id,share_cents,basis) VALUES(?,?,?,?)", id, uid, shares[uid], x.Mode); e != nil {
				break
			}
		}
	}
	if e == nil {
		_, e = tx.Exec("UPDATE transactions SET version=version+1 WHERE id=?", id)
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'transaction',?,'split')", u.ID, id)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		fail(w, 500, "Não foi possível ratear a despesa.")
		return
	}
	reply(w, 200, map[string]any{"ok": true, "shares": shares})
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

var _ = fmt.Sprintf
