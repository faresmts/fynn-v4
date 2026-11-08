package app

import (
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func parseCSVAmount(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "R$"))
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	f, e := strconv.ParseFloat(s, 64)
	return int64(f*100 + 0.5), e
}

func normalizeCSVDate(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"02/01/2006", "2006-01-02", "02-01-2006"} {
		if t, e := time.Parse(layout, s); e == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func (a *App) importCSV(w http.ResponseWriter, r *http.Request, u User) {
	if e := r.ParseMultipartForm(8 << 20); e != nil {
		fail(w, 400, "CSV inválido.")
		return
	}
	f, _, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, "Envie um arquivo CSV.")
		return
	}
	defer f.Close()
	accountID, e := strconv.ParseInt(strings.TrimSpace(r.FormValue("account_id")), 10, 64)
	if e != nil || accountID < 1 {
		fail(w, 400, "Escolha a conta dos lançamentos.")
		return
	}
	var categoryID *int64
	if v := strings.TrimSpace(r.FormValue("category_id")); v != "" {
		x, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			fail(w, 400, "Categoria inválida.")
			return
		}
		categoryID = &x
	}
	rd := csv.NewReader(io.LimitReader(f, 8<<20))
	rd.FieldsPerRecord = 3
	inserted, skipped := 0, 0
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível iniciar a importação.")
		return
	}
	defer tx.Rollback()
	first := true
	for {
		row, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(w, 422, "CSV inválido: use as colunas data, estabelecimento, valor.")
			return
		}
		if first {
			first = false
			if strings.EqualFold(strings.TrimSpace(row[0]), "data") {
				continue
			}
		}
		date := normalizeCSVDate(row[0])
		desc := strings.TrimSpace(row[1])
		amount, err := parseCSVAmount(row[2])
		if date == "" || desc == "" || err != nil || amount <= 0 {
			fail(w, 422, "Cada linha deve conter data, estabelecimento e valor válido.")
			return
		}
		var exists int
		if tx.QueryRow(`SELECT COUNT(*) FROM transactions WHERE owner_id=? AND account_id=? AND date=? AND description=? AND amount_cents=? AND type='expense'`, u.ID, accountID, date, desc, amount).Scan(&exists) != nil {
			fail(w, 500, "Falha ao verificar duplicidade.")
			return
		}
		if exists > 0 {
			skipped++
			continue
		}
		_, err = tx.Exec(`INSERT INTO transactions(household_id,owner_id,type,description,amount_cents,expected_cents,account_id,category_id,date,competence,due_date,status,visibility) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, u.HouseholdID, u.ID, "expense", desc, amount, amount, accountID, categoryID, date, date[:7], date, "pending", "private")
		if err != nil {
			fail(w, 500, "Falha ao inserir lançamento.")
			return
		}
		inserted++
	}
	if inserted == 0 && skipped == 0 {
		fail(w, 422, "O CSV não possui lançamentos.")
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, 500, "Falha ao concluir importação.")
		return
	}
	reply(w, 200, map[string]int{"inserted": inserted, "skipped": skipped})
}
