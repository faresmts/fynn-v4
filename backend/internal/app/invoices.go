package app

import (
	"database/sql"
	"errors"
	"fmt"
	"meu-financeiro/internal/domain"
	"net/http"
	"strconv"
)

type cardMonthTotal struct {
	ID        int64 `json:"card_id"`
	Purchases int64 `json:"purchase_cents"`
	Shared    int64 `json:"shared_cents"`
	Due       int64 `json:"due_cents"`
}

func (a *App) cardMonthTotals(u User, month string) ([]cardMonthTotal, error) {
	rows, err := a.DB.Query(`SELECT c.id,sum(l.amount_cents),
	sum(CASE WHEN s.line_id IS NOT NULL THEN l.amount_cents ELSE 0 END),
	sum(CASE WHEN i.status='open' AND coalesce(t.status,'pending')!='paid' THEN l.amount_cents ELSE 0 END)
	FROM cards c JOIN card_invoices i ON i.card_id=c.id JOIN card_installments l ON l.invoice_id=i.id
	LEFT JOIN shared_invoice_lines s ON s.line_id=l.id LEFT JOIN transactions t ON t.id=l.transaction_id
	WHERE c.owner_id=? AND c.household_id=? AND i.reference_month=? AND i.status!='cancelled' GROUP BY c.id`, u.ID, u.HouseholdID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []cardMonthTotal{}
	for rows.Next() {
		var x cardMonthTotal
		if err := rows.Scan(&x.ID, &x.Purchases, &x.Shared, &x.Due); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

type statementLine struct {
	csvExpense
	ID     int64 `json:"id"`
	Shared bool  `json:"shared"`
}

type statement struct {
	Month  string          `json:"month"`
	Status string          `json:"status"`
	Lines  []statementLine `json:"lines"`
	Total  int64           `json:"total_cents"`
	Due    int64           `json:"due_cents"`
}

func openInvoice(tx *sql.Tx, card int64, month string) (int64, error) {
	_, err := tx.Exec(`INSERT INTO card_invoices(card_id,reference_month,due_date,total_cents) VALUES(?,?,date(?||'-01','+1 month','-1 day'),0) ON CONFLICT(card_id,reference_month) DO NOTHING`, card, month, month)
	if err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRow("SELECT id FROM card_invoices WHERE card_id=? AND reference_month=? AND status='open'", card, month).Scan(&id)
	return id, err
}

func readStatement(tx *sql.Tx, owner User, card int64, month string) (statement, string, error) {
	s := statement{Month: month, Status: "open", Lines: []statementLine{}}
	var name string
	var status sql.NullString
	err := tx.QueryRow(`SELECT c.name,i.status FROM cards c LEFT JOIN card_invoices i ON i.card_id=c.id AND i.reference_month=? WHERE c.id=? AND c.owner_id=? AND c.household_id=? AND c.active=1`, month, card, owner.ID, owner.HouseholdID).Scan(&name, &status)
	if err != nil || !status.Valid {
		return s, name, err
	}
	s.Status = status.String
	rows, err := tx.Query(`SELECT l.id,EXISTS(SELECT 1 FROM shared_invoice_lines s WHERE s.line_id=l.id),l.purchase_date,l.description,l.amount_cents,coalesce(t.status,'pending')
 FROM card_installments l JOIN card_invoices i ON i.id=l.invoice_id LEFT JOIN transactions t ON t.id=l.transaction_id
 WHERE i.card_id=? AND i.reference_month=? ORDER BY l.purchase_date,l.id`, card, month)
	if err != nil {
		return s, name, err
	}
	defer rows.Close()
	for rows.Next() {
		var line statementLine
		var previousStatus string
		if err := rows.Scan(&line.ID, &line.Shared, &line.Date, &line.Description, &line.Amount, &previousStatus); err != nil {
			return s, name, err
		}
		s.Lines = append(s.Lines, line)
		s.Total += line.Amount
		if s.Status == "open" && previousStatus != "paid" {
			s.Due += line.Amount
		}
	}
	return s, name, rows.Err()
}

// The archived account is the card side of the transfer, outside the user's cash balances.
func cardClearingAccount(tx *sql.Tx, u User, card int64, name, date string) (int64, error) {
	var existing sql.NullInt64
	if err := tx.QueryRow("SELECT clearing_account_id FROM cards WHERE id=?", card).Scan(&existing); err != nil {
		return 0, err
	}
	if existing.Valid {
		return existing.Int64, nil
	}
	res, err := tx.Exec(`INSERT INTO accounts(household_id,owner_id,name,kind,opening_cents,opening_date,archived) VALUES(?,?,?,'checking',0,?,1)`, u.HouseholdID, u.ID, "Cartão "+name, date)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec("UPDATE cards SET clearing_account_id=? WHERE id=?", id, card)
	return id, err
}

func (a *App) cardInvoice(w http.ResponseWriter, r *http.Request, u User) {
	card, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	month, _, err := period(r)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "Falha ao abrir a fatura.")
		return
	}
	defer tx.Rollback()
	s, _, err := readStatement(tx, u, card, month)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "Cartão não encontrado.")
		return
	}
	if err != nil {
		fail(w, 500, "Falha ao consultar a fatura.")
		return
	}
	reply(w, 200, s)
}

func (a *App) payInvoice(w http.ResponseWriter, r *http.Request, u User) {
	card, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var in struct {
		Month   string `json:"month"`
		Account int64  `json:"account_id"`
		Amount  int64  `json:"amount_cents"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !domain.Month(in.Month) || in.Account < 1 || in.Amount <= 0 {
		fail(w, 400, "Confira o mês, a conta e o valor.")
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "Falha ao iniciar o pagamento.")
		return
	}
	defer tx.Rollback()
	s, name, err := readStatement(tx, u, card, in.Month)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "Cartão não encontrado.")
		return
	}
	if err != nil {
		fail(w, 500, "Falha ao ler a fatura.")
		return
	}
	if s.Status != "open" || s.Due != in.Amount {
		fail(w, 409, "O saldo da fatura mudou. Reabra a fatura antes de pagar.")
		return
	}
	date := today()
	var account int64
	if err = tx.QueryRow("SELECT id FROM accounts WHERE id=? AND owner_id=? AND household_id=? AND archived=0 AND opening_date<=?", in.Account, u.ID, u.HouseholdID, date).Scan(&account); err != nil {
		fail(w, 400, "Escolha uma conta de pagamento válida.")
		return
	}
	dest, err := cardClearingAccount(tx, u, card, name, date)
	if err != nil {
		fail(w, 500, "Falha ao preparar a transferência.")
		return
	}
	description := fmt.Sprintf("Fatura do cartão %s %s/%s", name, in.Month[5:], in.Month[:4])
	id, err := insertTransaction(tx, u, TransactionInput{Type: "transfer", Description: description, AccountID: account, DestinationID: &dest, Amount: s.Due, Date: date, Competence: date[:7], DueDate: date, PaidDate: &date, Status: "paid", Visibility: "private"})
	if err == nil {
		_, err = tx.Exec("UPDATE card_invoices SET status='paid',payment_transaction_id=? WHERE card_id=? AND reference_month=? AND status='open'", id, card, in.Month)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 500, "Falha ao pagar a fatura.")
		return
	}
	reply(w, 201, map[string]int64{"transaction_id": id})
}
