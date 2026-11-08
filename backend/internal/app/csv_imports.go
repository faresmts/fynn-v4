package app

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"meu-financeiro/internal/domain"
)

type csvExpense struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	Amount      int64  `json:"amount_cents"`
}

type csvReviewLine struct {
	csvExpense
	Duplicate bool `json:"duplicate"`
}

type csvReview struct {
	Lines     []csvReviewLine `json:"lines"`
	Total     int64           `json:"total_cents"`
	NewTotal  int64           `json:"new_cents"`
	New       int             `json:"new_count"`
	Duplicate int             `json:"duplicate_count"`
	Token     string          `json:"token"`
}

// A dot may group thousands in Brazilian notation or introduce decimal cents.
var csvCurrency = regexp.MustCompile(`^(?:[0-9]+(?:[,.][0-9]{1,2})?|[0-9]{1,3}(?:\.[0-9]{3})+(?:,[0-9]{1,2})?)$`)

func csvCents(raw string) (int64, error) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "R$"))
	if !csvCurrency.MatchString(s) {
		return 0, fmt.Errorf("valor monetário inválido")
	}
	if strings.Contains(s, ",") || len(s) >= 4 && s[len(s)-4] == '.' {
		s = strings.ReplaceAll(s, ".", "")
	}
	whole, frac, _ := strings.Cut(strings.ReplaceAll(s, ",", "."), ".")
	return strconv.ParseInt(whole+(frac + "00")[:2], 10, 64)
}

func csvDate(s, year string) string {
	if len(s) == 5 {
		s += "/" + year
	}
	for _, layout := range []string{"02/01/2006", "2006-01-02", "02-01-2006"} {
		if d, e := time.Parse(layout, s); e == nil && domain.Date(d.Format("2006-01-02")) {
			return d.Format("2006-01-02")
		}
	}
	return ""
}

func expenseCSV(data []byte, year string) (csvReview, error) {
	var review csvReview
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		return review, errors.New("Salve o CSV em UTF-8.")
	}
	var reader *csv.Reader
	for _, sep := range []rune{';', ','} {
		probe := csv.NewReader(bytes.NewReader(data))
		probe.Comma = sep
		if first, e := probe.Read(); e == nil && len(first) == 3 {
			reader = csv.NewReader(bytes.NewReader(data))
			reader.Comma = sep
			reader.FieldsPerRecord = 3
			break
		}
	}
	if reader == nil {
		return review, errors.New("Confira a primeira linha. Use três colunas separadas por ponto e vírgula ou vírgula.")
	}
	first := true
	hasFooter := false
	for {
		row, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return review, fmt.Errorf("Estrutura do CSV inválida (%v).", e)
		}
		line, _ := reader.FieldPos(0)
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		if first {
			first = false
			if strings.EqualFold(row[0], "data") {
				if !strings.EqualFold(row[1], "estabelecimento") || !strings.EqualFold(row[2], "valor") {
					return review, errors.New("Use o cabeçalho data, estabelecimento, valor, nessa ordem.")
				}
				continue
			}
		}
		if hasFooter {
			return review, fmt.Errorf("Linha %d: TOTAL deve encerrar a fatura.", line)
		}
		amount, e := csvCents(row[2])
		if e != nil {
			return review, fmt.Errorf("Linha %d: valor inválido.", line)
		}
		if row[0] == "" && strings.EqualFold(row[1], "TOTAL") {
			if amount != review.Total {
				return review, fmt.Errorf("Linha %d: o TOTAL diverge da soma dos gastos.", line)
			}
			hasFooter = true
			continue
		}
		date := csvDate(row[0], year)
		if date == "" || row[1] == "" || !domain.ValidAmount(amount) {
			return review, fmt.Errorf("Linha %d: confira a data, o estabelecimento e o valor positivo da despesa.", line)
		}
		review.Lines = append(review.Lines, csvReviewLine{csvExpense: csvExpense{Date: date, Description: row[1], Amount: amount}})
		review.Total += amount
	}
	if len(review.Lines) == 0 {
		return review, errors.New("O CSV não contém gastos.")
	}
	return review, nil
}

func existingCardExpenses(tx *sql.Tx, u User, cardID int64, month string) (map[csvExpense]int, error) {
	rows, e := tx.Query(`SELECT l.purchase_date,l.description,l.amount_cents,count(*) FROM card_installments l JOIN card_invoices i ON i.id=l.invoice_id JOIN cards c ON c.id=i.card_id WHERE c.household_id=? AND c.owner_id=? AND c.id=? AND i.reference_month=? GROUP BY l.purchase_date,l.description,l.amount_cents`, u.HouseholdID, u.ID, cardID, month)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	counts := make(map[csvExpense]int)
	for rows.Next() {
		var key csvExpense
		var n int
		if e := rows.Scan(&key.Date, &key.Description, &key.Amount, &n); e != nil {
			return nil, e
		}
		counts[key] = n
	}
	return counts, rows.Err()
}

func (a *App) csvImport(confirm bool) func(http.ResponseWriter, *http.Request, User) {
	return func(w http.ResponseWriter, r *http.Request, u User) {
		const maxCSV = 8 << 20
		r.Body = http.MaxBytesReader(w, r.Body, maxCSV+(1<<20))
		if e := r.ParseMultipartForm(maxCSV); e != nil {
			fail(w, 400, "Envie um arquivo CSV de até 8 MB.")
			return
		}
		f, h, e := r.FormFile("file")
		if e != nil {
			fail(w, 400, "Escolha o arquivo CSV.")
			return
		}
		defer f.Close()
		if h.Size > maxCSV {
			fail(w, 413, "O CSV excede 8 MB.")
			return
		}
		data, e := io.ReadAll(f)
		if e != nil {
			fail(w, 400, "Falha ao ler o CSV.")
			return
		}
		month := r.FormValue("month")
		if month == "" {
			month = today()[:7]
		}
		if !domain.Month(month) {
			fail(w, 400, "Mês da fatura inválido.")
			return
		}
		year := month[:4]
		review, e := expenseCSV(data, year)
		if e != nil {
			fail(w, 422, e.Error())
			return
		}
		cardID, e := strconv.ParseInt(r.FormValue("card_id"), 10, 64)
		if e != nil || cardID < 1 {
			fail(w, 400, "Escolha o cartão da fatura.")
			return
		}
		tx, e := a.DB.BeginTx(r.Context(), nil)
		if e != nil {
			fail(w, 500, "Falha ao abrir a fatura.")
			return
		}
		defer tx.Rollback()
		var valid bool
		e = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cards WHERE id=? AND household_id=? AND owner_id=? AND active=1)`, cardID, u.HouseholdID, u.ID).Scan(&valid)
		if e != nil || !valid {
			fail(w, 400, "Cartão inválido.")
			return
		}
		counts, e := existingCardExpenses(tx, u, cardID, month)
		if e != nil {
			fail(w, 500, "Falha ao conferir duplicidades.")
			return
		}
		for i := range review.Lines {
			line := &review.Lines[i]
			if counts[line.csvExpense] > 0 {
				line.Duplicate = true
				counts[line.csvExpense]--
				review.Duplicate++
				continue
			}
			review.New++
			review.NewTotal += line.Amount
		}
		// Bind confirmation to the reviewed file, choices and reconciliation state.
		fingerprint := fmt.Sprintf("%x/%d/%d/%s/%s/%d/%v", sha256.Sum256(data), u.ID, cardID, month, year, review.New, review.Lines)
		review.Token = fmt.Sprintf("%x", sha256.Sum256([]byte(fingerprint)))
		if !confirm {
			reply(w, 200, review)
			return
		}
		if r.FormValue("token") != review.Token {
			fail(w, 409, "A prévia mudou. Revise a fatura novamente.")
			return
		}
		invoiceID, e := openInvoice(tx, cardID, month)
		if e != nil {
			fail(w, 409, "A fatura está fechada ou indisponível.")
			return
		}
		for _, line := range review.Lines {
			if line.Duplicate {
				continue
			}
			_, e = tx.Exec(`INSERT INTO card_installments(invoice_id,description,amount_cents,purchase_date) VALUES(?,?,?,?)`, invoiceID, line.Description, line.Amount, line.Date)
			if e != nil {
				fail(w, 500, "Falha ao inserir os gastos da fatura.")
				return
			}
		}
		if _, e = tx.Exec("UPDATE card_invoices SET total_cents=total_cents+? WHERE id=?", review.NewTotal, invoiceID); e != nil {
			fail(w, 500, "Falha ao atualizar a fatura.")
			return
		}
		if e = tx.Commit(); e != nil {
			fail(w, 500, "Falha ao concluir a importação.")
			return
		}
		reply(w, 200, map[string]int{"inserted": review.New, "skipped": review.Duplicate})
	}
}
