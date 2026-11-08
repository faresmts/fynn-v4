package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"meu-financeiro/internal/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Account struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Opening int64  `json:"opening_cents"`
	Date    string `json:"opening_date"`
	Balance int64  `json:"balance_cents"`
}
type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}
type Transaction struct {
	InvoicePayment bool    `json:"invoice_payment"`
	ID             int64   `json:"id"`
	Type           string  `json:"type"`
	Description    string  `json:"description"`
	Amount         int64   `json:"amount_cents"`
	Expected       int64   `json:"expected_cents"`
	AccountID      int64   `json:"account_id"`
	DestinationID  *int64  `json:"destination_id"`
	CategoryID     *int64  `json:"category_id"`
	Date           string  `json:"date"`
	Competence     string  `json:"competence"`
	DueDate        string  `json:"due_date"`
	Status         string  `json:"status"`
	PaidDate       *string `json:"paid_date"`
	Visibility     string  `json:"visibility"`
	RuleID         *int64  `json:"recurring_rule_id"`
	Version        int     `json:"version"`
	Category       string  `json:"category"`
	Account        string  `json:"account"`
	OwnerID        int64   `json:"owner_id"`
}
type TransactionInput struct {
	Type          string  `json:"type"`
	Description   string  `json:"description"`
	Amount        int64   `json:"amount_cents"`
	AccountID     int64   `json:"account_id"`
	DestinationID *int64  `json:"destination_id"`
	CategoryID    *int64  `json:"category_id"`
	Date          string  `json:"date"`
	Competence    string  `json:"competence"`
	DueDate       string  `json:"due_date"`
	Status        string  `json:"status"`
	PaidDate      *string `json:"paid_date"`
	Visibility    string  `json:"visibility"`
}

func today() string {
	loc, _ := time.LoadLocation("America/Fortaleza")
	return time.Now().In(loc).Format("2006-01-02")
}
func textValid(s string, max int) bool { return len(strings.TrimSpace(s)) > 0 && len(s) <= max }
func (a *App) accountList(u User) ([]Account, error) {
	rows, e := a.DB.Query(`SELECT a.id,a.name,a.kind,a.opening_cents,a.opening_date,a.opening_cents+COALESCE((SELECT SUM(CASE WHEN t.type='income' OR t.destination_id=a.id THEN t.amount_cents ELSE -t.amount_cents END) FROM transactions t WHERE (t.account_id=a.id OR t.destination_id=a.id) AND t.status='paid' AND t.paid_date<=?),0) FROM accounts a WHERE a.owner_id=? AND a.household_id=? AND a.archived=0 ORDER BY a.id`, today(), u.ID, u.HouseholdID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		var x Account
		if e = rows.Scan(&x.ID, &x.Name, &x.Kind, &x.Opening, &x.Date, &x.Balance); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (a *App) accounts(w http.ResponseWriter, r *http.Request, u User) {
	v, e := a.accountList(u)
	if e != nil {
		fail(w, 500, "Não foi possível carregar as contas.")
		return
	}
	reply(w, 200, v)
}
func (a *App) createAccount(w http.ResponseWriter, r *http.Request, u User) {
	var x struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Opening int64  `json:"opening_cents"`
		Date    string `json:"opening_date"`
	}
	if !decode(w, r, &x) {
		return
	}
	if !textValid(x.Name, 80) || (x.Kind != "checking" && x.Kind != "savings" && x.Kind != "cash") || x.Opening > domain.MaxCents || x.Opening < -domain.MaxCents || !domain.Date(x.Date) || x.Date > today() {
		fail(w, 400, "Confira o nome, tipo, saldo e data de abertura da conta.")
		return
	}
	res, e := a.DB.Exec("INSERT INTO accounts(household_id,owner_id,name,kind,opening_cents,opening_date) VALUES(?,?,?,?,?,?)", u.HouseholdID, u.ID, strings.TrimSpace(x.Name), x.Kind, x.Opening, x.Date)
	if e != nil {
		fail(w, 500, "Não foi possível criar a conta.")
		return
	}
	id, _ := res.LastInsertId()
	reply(w, 201, map[string]int64{"id": id})
}
func (a *App) categories(w http.ResponseWriter, r *http.Request, u User) {
	rows, e := a.DB.Query("SELECT id,name,type FROM categories WHERE household_id=? ORDER BY name", u.HouseholdID)
	if e != nil {
		fail(w, 500, "Não foi possível carregar as categorias.")
		return
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if e = rows.Scan(&c.ID, &c.Name, &c.Type); e != nil {
			fail(w, 500, "Falha na leitura.")
			return
		}
		out = append(out, c)
	}
	if rows.Err() != nil {
		fail(w, 500, "Falha na leitura.")
		return
	}
	reply(w, 200, out)
}
func (a *App) createCategory(w http.ResponseWriter, r *http.Request, u User) {
	var x struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if !decode(w, r, &x) {
		return
	}
	if !textValid(x.Name, 60) || (x.Type != "income" && x.Type != "expense") {
		fail(w, 400, "Informe o nome e o tipo da categoria.")
		return
	}
	res, e := a.DB.Exec("INSERT INTO categories(household_id,name,type) VALUES(?,?,?)", u.HouseholdID, strings.TrimSpace(x.Name), x.Type)
	if e != nil {
		fail(w, 409, "Não foi possível criar. Verifique se a categoria já existe.")
		return
	}
	id, _ := res.LastInsertId()
	reply(w, 201, map[string]int64{"id": id})
}
func validateTransaction(tx *sql.Tx, u User, x TransactionInput) error {
	if !domain.ValidAmount(x.Amount) || !textValid(x.Description, 180) || !domain.Date(x.Date) || !domain.Month(x.Competence) || !domain.Date(x.DueDate) || (x.Status != "pending" && x.Status != "paid") || (x.Visibility != "private" && x.Visibility != "household") || (x.Type != "income" && x.Type != "expense" && x.Type != "transfer") {
		return fmt.Errorf("Confira valor, descrição, datas e estado do lançamento.")
	}
	var opening string
	if e := tx.QueryRow("SELECT opening_date FROM accounts WHERE id=? AND owner_id=? AND household_id=? AND archived=0", x.AccountID, u.ID, u.HouseholdID).Scan(&opening); e != nil {
		return fmt.Errorf("Selecione uma conta sua.")
	}
	if x.Date < opening {
		return fmt.Errorf("A data deve ser posterior ou igual à abertura da conta.")
	}
	if x.Status == "paid" {
		if x.PaidDate == nil || !domain.Date(*x.PaidDate) || *x.PaidDate < opening || *x.PaidDate > today() {
			return fmt.Errorf("Informe uma data de pagamento válida, até hoje.")
		}
	} else if x.PaidDate != nil {
		return fmt.Errorf("Um lançamento pendente não possui pagamento.")
	}
	if x.Type == "transfer" {
		if x.DestinationID == nil || *x.DestinationID == x.AccountID || x.CategoryID != nil || x.Status != "paid" || x.Visibility != "private" {
			return fmt.Errorf("Transferências exigem duas contas diferentes e pagamento confirmado.")
		}
		if e := tx.QueryRow("SELECT opening_date FROM accounts WHERE id=? AND owner_id=? AND household_id=? AND archived=0", *x.DestinationID, u.ID, u.HouseholdID).Scan(&opening); e != nil {
			return fmt.Errorf("Selecione uma conta de destino sua.")
		}
		if x.Date < opening || *x.PaidDate < opening {
			return fmt.Errorf("A transferência antecede a abertura da conta de destino.")
		}
	} else {
		if x.DestinationID != nil || x.CategoryID == nil {
			return fmt.Errorf("Selecione uma categoria.")
		}
		var n int
		if e := tx.QueryRow("SELECT count(*) FROM categories WHERE id=? AND household_id=? AND type=?", x.CategoryID, u.HouseholdID, x.Type).Scan(&n); e != nil || n != 1 {
			return fmt.Errorf("Categoria incompatível com o lançamento.")
		}
	}
	return nil
}
func insertTransaction(tx *sql.Tx, u User, x TransactionInput) (int64, error) {
	res, e := tx.Exec(`INSERT INTO transactions(household_id,owner_id,type,description,amount_cents,expected_cents,account_id,destination_id,category_id,date,competence,due_date,status,paid_date,visibility) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, u.HouseholdID, u.ID, x.Type, strings.TrimSpace(x.Description), x.Amount, x.Amount, x.AccountID, x.DestinationID, x.CategoryID, x.Date, x.Competence, x.DueDate, x.Status, x.PaidDate, x.Visibility)
	if e != nil {
		return 0, e
	}
	id, e := res.LastInsertId()
	if e != nil {
		return 0, e
	}
	_, e = tx.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'transaction',?,'create')", u.ID, id)
	return id, e
}
func (a *App) createTransaction(w http.ResponseWriter, r *http.Request, u User) {
	var x TransactionInput
	if !decode(w, r, &x) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 16 || len(key) > 100 {
		fail(w, 400, "Chave de idempotência obrigatória (16 a 100 caracteres).")
		return
	}
	raw, _ := json.Marshal(x)
	fingerprint := digest(string(raw))
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível salvar.")
		return
	}
	defer tx.Rollback()
	var oldID int64
	var oldHash string
	e = tx.QueryRow("SELECT transaction_id,fingerprint FROM idempotency WHERE user_id=? AND key=?", u.ID, key).Scan(&oldID, &oldHash)
	if e == nil {
		if oldHash != fingerprint {
			fail(w, 409, "A chave já foi usada para outro lançamento.")
			return
		}
		reply(w, 200, map[string]int64{"id": oldID})
		return
	}
	if e != sql.ErrNoRows {
		fail(w, 500, "Não foi possível salvar.")
		return
	}
	if e = validateTransaction(tx, u, x); e != nil {
		fail(w, 400, e.Error())
		return
	}
	id, e := insertTransaction(tx, u, x)
	if e == nil {
		_, e = tx.Exec("INSERT INTO idempotency(user_id,key,fingerprint,transaction_id) VALUES(?,?,?,?)", u.ID, key, fingerprint, id)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		fail(w, 500, "Não foi possível salvar o lançamento.")
		return
	}
	reply(w, 201, map[string]int64{"id": id})
}

const transactionSelect = `SELECT t.id,t.type,t.description,t.amount_cents,t.expected_cents,t.account_id,t.destination_id,t.category_id,t.date,t.competence,t.due_date,t.status,t.paid_date,t.visibility,t.recurring_rule_id,t.version,CASE WHEN i.id IS NOT NULL THEN 'Pagamento de fatura' ELSE COALESCE(c.name,'Transferência') END,a.name,t.owner_id,i.id IS NOT NULL FROM transactions t JOIN accounts a ON a.id=t.account_id LEFT JOIN categories c ON c.id=t.category_id LEFT JOIN card_invoices i ON i.payment_transaction_id=t.id `

func (a *App) transactionList(u User, month, context string) ([]Transaction, error) {
	query := transactionSelect + `WHERE t.card_id IS NULL AND t.household_id=? AND `
	args := []any{u.HouseholdID}
	if context == "household" {
		query += "t.visibility='household'"
	} else {
		query += "t.owner_id=?"
		args = append(args, u.ID)
	}
	if month != "" {
		query += " AND t.competence=?"
		args = append(args, month)
	}
	query += " ORDER BY t.date DESC,t.id DESC"
	rows, e := a.DB.Query(query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Transaction{}
	for rows.Next() {
		var t Transaction
		if e = rows.Scan(&t.ID, &t.Type, &t.Description, &t.Amount, &t.Expected, &t.AccountID, &t.DestinationID, &t.CategoryID, &t.Date, &t.Competence, &t.DueDate, &t.Status, &t.PaidDate, &t.Visibility, &t.RuleID, &t.Version, &t.Category, &t.Account, &t.OwnerID, &t.InvoicePayment); e != nil {
			return nil, e
		}
		if t.OwnerID != u.ID {
			t.Account = "Conta privada"
			t.AccountID = 0
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func period(r *http.Request) (string, string, error) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = today()[:7]
	}
	ctx := r.URL.Query().Get("context")
	if ctx == "" {
		ctx = "personal"
	}
	if !domain.Month(month) || (ctx != "personal" && ctx != "household") {
		return "", "", fmt.Errorf("Período ou contexto inválido.")
	}
	return month, ctx, nil
}
func (a *App) transactions(w http.ResponseWriter, r *http.Request, u User) {
	month, ctx, e := period(r)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	out, e := a.transactionList(u, month, ctx)
	if e != nil {
		fail(w, 500, "Não foi possível carregar os lançamentos.")
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	total := len(out)
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	reply(w, 200, map[string]any{"items": out[offset:end], "total": total, "offset": offset, "limit": limit})
}
func (a *App) updateTransaction(w http.ResponseWriter, r *http.Request, u User) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		fail(w, 404, "Lançamento não encontrado.")
		return
	}
	var x struct {
		Version     int    `json:"version"`
		Action      string `json:"action"`
		Amount      int64  `json:"amount_cents"`
		PaidDate    string `json:"paid_date"`
		Description string `json:"description"`
	}
	if !decode(w, r, &x) {
		return
	}
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível atualizar.")
		return
	}
	defer tx.Rollback()
	var status, kind, opening string
	var version int
	e = tx.QueryRow("SELECT t.status,t.version,t.type,a.opening_date FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE t.card_id IS NULL AND NOT EXISTS(SELECT 1 FROM card_invoices i WHERE i.payment_transaction_id=t.id) AND t.id=? AND t.owner_id=? AND t.household_id=?", id, u.ID, u.HouseholdID).Scan(&status, &version, &kind, &opening)
	if e != nil {
		fail(w, 404, "Lançamento não encontrado.")
		return
	}
	if version != x.Version {
		fail(w, 409, "Este lançamento mudou. Atualize a página e tente novamente.")
		return
	}
	if status == "cancelled" {
		fail(w, 409, "O lançamento já está cancelado.")
		return
	}
	switch x.Action {
	case "cancel":
		_, e = tx.Exec("UPDATE transactions SET status='cancelled',version=version+1 WHERE id=?", id)
	case "pay":
		if status != "pending" || !domain.ValidAmount(x.Amount) || !domain.Date(x.PaidDate) || x.PaidDate < opening || x.PaidDate > today() {
			fail(w, 400, "Confira o valor e a data de pagamento.")
			return
		}
		_, e = tx.Exec("UPDATE transactions SET amount_cents=?,paid_date=?,status='paid',version=version+1 WHERE id=?", x.Amount, x.PaidDate, id)
	case "edit":
		if status != "pending" || !domain.ValidAmount(x.Amount) || !textValid(x.Description, 180) {
			fail(w, 400, "Somente pendentes podem ser editados. Confira descrição e valor.")
			return
		}
		_, e = tx.Exec("UPDATE transactions SET amount_cents=?,description=?,version=version+1 WHERE id=?", x.Amount, strings.TrimSpace(x.Description), id)
	case "share":
		_, e = tx.Exec("UPDATE transactions SET visibility='household',version=version+1 WHERE id=? AND visibility='private'", id)
	case "unshare":
		_, e = tx.Exec("UPDATE transactions SET visibility='private',version=version+1 WHERE id=? AND visibility='household'", id)
	default:
		fail(w, 400, "Ação inválida.")
		return
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'transaction',?,?)", u.ID, id, x.Action)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		fail(w, 500, "Não foi possível atualizar.")
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
