package app

import (
	"database/sql"
	"meu-financeiro/internal/domain"
	"net/http"
	"strings"
)

type financing struct {
	Payments     []financingPayment `json:"payments"`
	ID           int64              `json:"id"`
	Name         string             `json:"name"`
	Total        int64              `json:"total_cents"`
	Paid         int64              `json:"paid_cents"`
	Installments int                `json:"installments"`
	Installment  int64              `json:"installment_cents"`
}
type financingPayment struct {
	ID     int64  `json:"id"`
	Date   string `json:"paid_on"`
	Amount int64  `json:"amount_cents"`
}

func (a *App) financings(w http.ResponseWriter, r *http.Request, u User) {
	rows, err := a.DB.Query(`SELECT f.id,f.name,f.total_cents,f.paid_cents,f.installments,f.installment_cents,
	coalesce(p.id,0),coalesce(p.paid_on,''),coalesce(p.amount_cents,0) FROM financings f
	LEFT JOIN financing_payments p ON p.financing_id=f.id WHERE f.owner_id=? ORDER BY f.id DESC,p.paid_on DESC,p.id DESC`, u.ID)
	if err != nil {
		fail(w, 500, "Falha ao carregar financiamentos.")
		return
	}
	defer rows.Close()
	items := []financing{}
	for rows.Next() {
		var f financing
		var payment financingPayment
		if err = rows.Scan(&f.ID, &f.Name, &f.Total, &f.Paid, &f.Installments, &f.Installment, &payment.ID, &payment.Date, &payment.Amount); err != nil {
			fail(w, 500, "Erro de leitura.")
			return
		}
		if len(items) == 0 || items[len(items)-1].ID != f.ID {
			f.Payments = []financingPayment{}
			items = append(items, f)
		}
		if payment.ID != 0 {
			contract := &items[len(items)-1]
			contract.Payments = append(contract.Payments, payment)
		}
	}
	if rows.Err() != nil {
		fail(w, 500, "Erro de leitura.")
		return
	}
	reply(w, 200, items)
}
func (a *App) createFinancing(w http.ResponseWriter, r *http.Request, u User) {
	var f financing
	if !decode(w, r, &f) {
		return
	}
	f.Name = strings.TrimSpace(f.Name)
	if !textValid(f.Name, 100) || !domain.ValidAmount(f.Total) || f.Paid < 0 || f.Paid > f.Total || f.Installments < 1 || !domain.ValidAmount(f.Installment) {
		fail(w, 400, "Confira os valores e a quantidade de parcelas.")
		return
	}
	result, err := a.DB.Exec("INSERT INTO financings(owner_id,name,total_cents,paid_cents,installments,installment_cents) VALUES(?,?,?,?,?,?)", u.ID, f.Name, f.Total, f.Paid, f.Installments, f.Installment)
	if err != nil {
		fail(w, 500, "Falha ao cadastrar financiamento.")
		return
	}
	f.ID, _ = result.LastInsertId()
	f.Payments = []financingPayment{}
	reply(w, 201, f)
}
func (a *App) payFinancing(w http.ResponseWriter, r *http.Request, u User) {
	var input struct {
		Paid *int64 `json:"paid_cents"`
		Date string `json:"paid_on"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Paid == nil {
		fail(w, 400, "Informe o valor já pago.")
		return
	}
	var paid int64
	if input.Date == "" {
		input.Date = today()
	}
	if !domain.Date(input.Date) || input.Date > today() {
		fail(w, 400, "Informe uma data de pagamento até hoje.")
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "Falha ao iniciar pagamento.")
		return
	}
	defer tx.Rollback()
	err = tx.QueryRow(`UPDATE financings SET paid_cents=MIN(total_cents,paid_cents+installment_cents)
 WHERE id=? AND owner_id=? AND paid_cents=? AND paid_cents<total_cents RETURNING paid_cents`, r.PathValue("id"), u.ID, *input.Paid).Scan(&paid)
	if err == sql.ErrNoRows {
		fail(w, 409, "O financiamento está quitado, indisponível ou mudou. Confira os valores atualizados.")
		return
	}
	if err != nil {
		fail(w, 500, "Falha ao registrar pagamento.")
		return
	}
	_, err = tx.Exec("INSERT INTO financing_payments(financing_id,paid_on,amount_cents) VALUES(?,?,?)", r.PathValue("id"), input.Date, paid-*input.Paid)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 500, "Falha ao gravar o histórico.")
		return
	}
	reply(w, 200, map[string]int64{"paid_cents": paid})
}
func (a *App) deleteFinancingPayment(w http.ResponseWriter, r *http.Request, u User) {
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "Falha ao iniciar exclusão.")
		return
	}
	defer tx.Rollback()
	var amount int64
	err = tx.QueryRow(`DELETE FROM financing_payments WHERE id=? AND financing_id=?
	AND financing_id IN (SELECT id FROM financings WHERE owner_id=?) RETURNING amount_cents`, r.PathValue("payment"), r.PathValue("id"), u.ID).Scan(&amount)
	if err == sql.ErrNoRows {
		fail(w, 404, "Pagamento não encontrado.")
		return
	}
	if err == nil {
		_, err = tx.Exec("UPDATE financings SET paid_cents=paid_cents-? WHERE id=? AND owner_id=?", amount, r.PathValue("id"), u.ID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 500, "Falha ao excluir o pagamento.")
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
