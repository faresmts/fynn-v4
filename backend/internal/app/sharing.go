package app

import (
	"database/sql"
	"errors"
	"math/bits"
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
	allConfirmed := true
	for _, p := range people {
		allConfirmed = allConfirmed && p.Confirmed
	}
	if total == 0 || !allConfirmed {
		reply(w, 200, map[string]any{"month": month, "status": "needs_rule", "people": people, "total_income_cents": int64(0), "message": "Confirme as rendas ou escolha divisão manual antes de ratear."})
		return
	}
	reply(w, 200, map[string]any{"month": month, "status": "ready", "people": people, "total_income_cents": total})
}

func (a *App) shareInvoiceLine(w http.ResponseWriter, r *http.Request, u User) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		fail(w, 404, "Compra não encontrada.")
		return
	}
	// The reference never posts a transaction. Repeated requests preserve any saved split.
	res, e := a.DB.Exec(`INSERT INTO shared_invoice_lines(line_id)
 SELECT l.id FROM card_installments l JOIN card_invoices i ON i.id=l.invoice_id JOIN cards c ON c.id=i.card_id
 WHERE l.id=? AND c.owner_id=? AND c.household_id=? AND i.status!='cancelled'
 ON CONFLICT(line_id) DO NOTHING`, id, u.ID, u.HouseholdID)
	if e != nil {
		fail(w, 500, "Falha ao compartilhar a compra.")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var shared bool
		e = a.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM household_costs WHERE source='invoice' AND id=? AND owner_id=? AND household_id=?)", id, u.ID, u.HouseholdID).Scan(&shared)
		if e != nil || !shared {
			fail(w, 404, "Compra não encontrada.")
			return
		}
	}
	reply(w, 200, map[string]bool{"shared": true})
}

type resident struct {
	ID        int64
	Name      string
	Income    int64
	Confirmed bool
}

func residents(tx *sql.Tx, u User, month string) ([]resident, error) {
	rows, e := tx.Query(`SELECT u.id,u.name,coalesce(s.income_cents,0),coalesce(s.confirmed,0) FROM users u
 LEFT JOIN income_snapshots s ON s.user_id=u.id AND s.household_id=u.household_id AND s.month=? WHERE u.household_id=? ORDER BY u.id`, month, u.HouseholdID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var result []resident
	for rows.Next() {
		var p resident
		if e := rows.Scan(&p.ID, &p.Name, &p.Income, &p.Confirmed); e != nil {
			return nil, e
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func allocateSplit(amount int64, people []resident, mode string, custom map[int64]int64) (map[int64]int64, error) {
	shares := make(map[int64]int64)
	var totalIncome, allocated int64
	for _, p := range people {
		totalIncome += p.Income
	}
	for i, p := range people {
		var part int64
		switch mode {
		case "equal":
			part = amount / int64(len(people))
		case "monthly_income_ratio":
			if !p.Confirmed || totalIncome == 0 {
				return nil, errors.New("Confirme as rendas do mês antes de usar a divisão proporcional.")
			}
			hi, lo := bits.Mul64(uint64(amount), uint64(p.Income))
			quotient, _ := bits.Div64(hi, lo, uint64(totalIncome))
			part = int64(quotient)
		case "custom":
			v, ok := custom[p.ID]
			if !ok || v < 0 || v > amount-allocated {
				return nil, errors.New("Confira os valores de cada morador.")
			}
			part = v
		default:
			return nil, errors.New("Regra de divisão inválida.")
		}
		if mode != "custom" && i == len(people)-1 {
			part = amount - allocated
		}
		shares[p.ID] = part
		allocated += part
	}
	if allocated != amount || (mode == "custom" && len(custom) != len(people)) {
		return nil, errors.New("As cotas devem somar o valor da despesa.")
	}
	return shares, nil
}

func (a *App) splitExpense(source string) func(http.ResponseWriter, *http.Request, User) {
	return func(w http.ResponseWriter, r *http.Request, u User) {
		id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if e != nil {
			fail(w, 404, "Despesa não encontrada.")
			return
		}
		var x struct {
			Version int             `json:"version"`
			Mode    string          `json:"mode"`
			Custom  map[int64]int64 `json:"custom_cents"`
		}
		if !decode(w, r, &x) {
			return
		}
		tx, e := a.DB.BeginTx(r.Context(), nil)
		if e != nil {
			fail(w, 500, "Falha ao iniciar o rateio.")
			return
		}
		defer tx.Rollback()
		var amount int64
		var version int
		var month string
		e = tx.QueryRow("SELECT amount_cents,version,competence FROM household_costs WHERE source=? AND id=? AND household_id=?", source, id, u.HouseholdID).Scan(&amount, &version, &month)
		if e != nil {
			fail(w, 404, "Despesa compartilhada não encontrada.")
			return
		}
		if version != x.Version {
			fail(w, 409, "Esta despesa mudou. Atualize a página.")
			return
		}
		people, e := residents(tx, u, month)
		if e != nil {
			fail(w, 500, "Falha ao consultar moradores.")
			return
		}
		shares, e := allocateSplit(amount, people, x.Mode, x.Custom)
		if e != nil {
			fail(w, 400, e.Error())
			return
		}
		table, key, update := "transaction_shares", "transaction_id", "UPDATE transactions SET version=version+1 WHERE id=?"
		if source == "invoice" {
			table, key, update = "invoice_line_shares", "line_id", "UPDATE shared_invoice_lines SET version=version+1 WHERE line_id=?"
		}
		_, e = tx.Exec("DELETE FROM "+table+" WHERE "+key+"=?", id)
		for _, p := range people {
			if e != nil {
				break
			}
			_, e = tx.Exec("INSERT INTO "+table+"("+key+",participant_id,share_cents,basis) VALUES(?,?,?,?)", id, p.ID, shares[p.ID], x.Mode)
		}
		if e == nil {
			_, e = tx.Exec(update, id)
		}
		if e == nil {
			_, e = tx.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,?,?,'split')", u.ID, source, id)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			fail(w, 500, "Falha ao salvar o rateio.")
			return
		}
		reply(w, 200, map[string]any{"ok": true, "shares": shares})
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
