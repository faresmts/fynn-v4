package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CardImportLine struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Amount      int64  `json:"amount_cents"`
	CardLast4   string `json:"card_last4,omitempty"`
	Installment string `json:"installment,omitempty"`
}
type CardImportPreview struct {
	ID             int64            `json:"id"`
	Institution    string           `json:"institution"`
	Filename       string           `json:"filename"`
	StatementMonth string           `json:"statement_month"`
	Lines          []CardImportLine `json:"lines"`
	Total          int64            `json:"total_cents"`
}

var picpayLine = regexp.MustCompile(`^\s*(\d{2}/\d{2})\s+(.+?)\s+(-?[0-9.]+,[0-9]{2})\s*$`)
var nubankLine = regexp.MustCompile(`^\s*(\d{2})\s+([A-Z]{3})\s+•{2,4}\s*(\d{4})\s+(.+?)\s+([0-9.]+,[0-9]{2})\s*$`)
var nubankMonths = map[string]string{"JAN": "01", "FEV": "02", "MAR": "03", "ABR": "04", "MAI": "05", "JUN": "06", "JUL": "07", "AGO": "08", "SET": "09", "OUT": "10", "NOV": "11", "DEZ": "12"}

func centsBR(s string) (int64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	f, e := strconv.ParseFloat(s, 64)
	return int64(f*100 + 0.5), e
}
func pdfText(path string) (string, error) {
	var out bytes.Buffer
	c := exec.Command("pdftotext", "-layout", path, "-")
	c.Stdout = &out
	c.Stderr = io.Discard
	if e := c.Run(); e != nil {
		return "", e
	}
	return out.String(), nil
}
func parseCardPDF(text string) (string, string, []CardImportLine, error) {
	year := time.Now().Year()
	institution := ""
	month := ""
	lines := []CardImportLine{}
	if strings.Contains(strings.ToLower(text), "picpay mastercard") {
		institution = "picpay"
		re := regexp.MustCompile(`Vencimento:\s*\d{2}/(\d{2})/(\d{4})`)
		if m := re.FindStringSubmatch(text); len(m) > 0 {
			month = m[3] + "-" + m[2]
			year, _ = strconv.Atoi(m[3])
		}
	} else if strings.Contains(text, "Nu Pagamentos S.A.") {
		institution = "nubank"
		re := regexp.MustCompile(`FATURA\s+\d{2}\s+([A-Z]{3})\s+(\d{4})`)
		if m := re.FindStringSubmatch(text); len(m) > 0 {
			year, _ = strconv.Atoi(m[2])
			month = fmt.Sprintf("%04d-%s", year, nubankMonths[m[1]])
		}
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if institution == "picpay" {
			m := picpayLine.FindStringSubmatch(line)
			if len(m) == 0 || strings.HasPrefix(strings.ToUpper(m[2]), "PAGAMENTO DE FATURA") || strings.HasPrefix(strings.ToUpper(m[2]), "IOF") {
				continue
			}
			c, e := centsBR(m[3])
			if e != nil || c <= 0 {
				continue
			}
			p := strings.Split(m[1], "/")
			d := fmt.Sprintf("%04d-%s-%s", year, p[1], p[0])
			desc := strings.TrimSpace(m[2])
			inst := ""
			if x := regexp.MustCompile(`PARC\d{2}/\d{2}`).FindString(desc); x != "" {
				inst = x
			}
			lines = append(lines, CardImportLine{Date: d, Description: desc, Amount: c, Installment: inst})
		} else if institution == "nubank" {
			m := nubankLine.FindStringSubmatch(line)
			if len(m) == 0 {
				continue
			}
			c, e := centsBR(m[5])
			if e != nil || c <= 0 {
				continue
			}
			mon := nubankMonths[m[2]]
			d := fmt.Sprintf("%04d-%s-%s", year, mon, m[1])
			lines = append(lines, CardImportLine{Date: d, Description: strings.TrimSpace(m[4]), Amount: c, CardLast4: m[3]})
		}
	}
	if institution == "" || len(lines) == 0 {
		return "", "", nil, fmt.Errorf("não reconheci lançamentos neste PDF")
	}
	return institution, month, lines, nil
}

func (a *App) cardImportPreview(w http.ResponseWriter, r *http.Request, u User) {
	if e := r.ParseMultipartForm(12 << 20); e != nil {
		fail(w, 400, "Arquivo inválido.")
		return
	}
	f, h, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, "Envie um PDF no campo file.")
		return
	}
	defer f.Close()
	tmp, e := os.CreateTemp("", "fynn-card-*.pdf")
	if e != nil {
		fail(w, 500, "Não foi possível preparar o arquivo.")
		return
	}
	p := tmp.Name()
	defer os.Remove(p)
	if _, e = io.CopyN(tmp, f, 10<<20); e != nil && e != io.EOF {
		fail(w, 400, "PDF muito grande.")
		return
	}
	tmp.Close()
	text, e := pdfText(p)
	if e != nil {
		fail(w, 422, "Não foi possível ler o PDF.")
		return
	}
	inst, month, lines, e := parseCardPDF(text)
	if e != nil {
		fail(w, 422, e.Error())
		return
	}
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Falha ao iniciar importação.")
		return
	}
	res, e := tx.Exec(`INSERT INTO card_imports(household_id,owner_id,institution,filename,statement_month) VALUES(?,?,?,?,?)`, u.HouseholdID, u.ID, inst, h.Filename, month)
	if e != nil {
		tx.Rollback()
		fail(w, 500, "Falha ao salvar importação.")
		return
	}
	id, _ := res.LastInsertId()
	total := int64(0)
	for i := range lines {
		_, e = tx.Exec(`INSERT OR IGNORE INTO card_import_lines(import_id,purchase_date,description,amount_cents,card_last4,installment) VALUES(?,?,?,?,?,?)`, id, lines[i].Date, lines[i].Description, lines[i].Amount, lines[i].CardLast4, lines[i].Installment)
		lines[i].ID = int64(i + 1)
		total += lines[i].Amount
	}
	if e = tx.Commit(); e != nil {
		fail(w, 500, "Falha ao salvar importação.")
		return
	}
	reply(w, 200, CardImportPreview{ID: id, Institution: inst, Filename: h.Filename, StatementMonth: month, Lines: lines, Total: total})
}

func (a *App) cardImportConfirm(w http.ResponseWriter, r *http.Request, u User) {
	var in struct {
		ImportID   int64  `json:"import_id"`
		AccountID  int64  `json:"account_id"`
		CategoryID *int64 `json:"category_id"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.ImportID < 1 || in.AccountID < 1 {
		fail(w, 400, "Importação e conta são obrigatórias.")
		return
	}
	rows, e := a.DB.Query(`SELECT purchase_date,description,amount_cents FROM card_import_lines WHERE import_id=? AND transaction_id IS NULL ORDER BY id`, in.ImportID)
	if e != nil {
		fail(w, 500, "Falha ao carregar importação.")
		return
	}
	defer rows.Close()
	type x struct {
		d, desc string
		c       int64
	}
	all := []x{}
	for rows.Next() {
		var z x
		if rows.Scan(&z.d, &z.desc, &z.c) == nil {
			all = append(all, z)
		}
	}
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Falha ao confirmar.")
		return
	}
	for _, z := range all {
		res, e := tx.Exec(`INSERT INTO transactions(household_id,owner_id,type,description,amount_cents,expected_cents,account_id,category_id,date,competence,due_date,status,visibility) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, u.HouseholdID, u.ID, "expense", z.desc, z.c, z.c, in.AccountID, in.CategoryID, z.d, z.d[:7], z.d, "pending", "private")
		if e != nil {
			tx.Rollback()
			fail(w, 500, "Falha ao criar lançamentos.")
			return
		}
		id, _ := res.LastInsertId()
		_, e = tx.Exec(`UPDATE card_import_lines SET transaction_id=? WHERE import_id=? AND purchase_date=? AND description=? AND amount_cents=?`, id, in.ImportID, z.d, z.desc, z.c)
		if e != nil {
			tx.Rollback()
			fail(w, 500, "Falha ao finalizar importação.")
			return
		}
	}
	tx.Exec(`UPDATE card_imports SET confirmed_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=? AND owner_id=?`, in.ImportID, u.ID)
	if e = tx.Commit(); e != nil {
		fail(w, 500, "Falha ao finalizar importação.")
		return
	}
	reply(w, 200, map[string]int{"imported": len(all)})
}
