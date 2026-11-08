package app

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func (f *fixture) invoiceRequest(path, csv, card, token string) *httptest.ResponseRecorder {
	f.t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, e := form.CreateFormFile("file", "fatura.csv")
	if e != nil {
		f.t.Fatal(e)
	}
	part.Write([]byte(csv))
	form.WriteField("card_id", card)
	form.WriteField("token", token)
	form.Close()
	r := httptest.NewRequest("POST", "/api/v1/imports/csv"+path, &body)
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("X-CSRF-Token", f.csrf)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.AddCookie(f.cookie)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func TestCSVReviewAndConfirm(t *testing.T) {
	data := "\ufeffdata;estabelecimento;valor\r\n09/02;Loja;64,50\r\n09/02;Loja;64,50\r\n;TOTAL;129,00\r\n"
	wantCount, wantTotal := 2, int64(12900)
	// Exercise the supplied statement locally without storing personal data in fixtures.
	if path := os.Getenv("PICPAY_TEST_CSV"); path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		data = string(b)
		wantCount, wantTotal = 51, 459133
	}
	f := setup(t)
	if _, e := f.db.Exec(`INSERT INTO cards(id,household_id,owner_id,institution,name) VALUES (1,1,1,'picpay','A'),(2,1,1,'picpay','B'),(3,1,2,'picpay','C')`); e != nil {
		t.Fatal(e)
	}
	preview := func(card string) csvReview {
		t.Helper()
		w := f.invoiceRequest("", data, card, "")
		var p csvReview
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil {
			t.Fatalf("preview: %d", w.Code)
		}
		return p
	}
	p := preview("1")
	if len(p.Lines) != wantCount || p.New != wantCount || p.NewTotal != wantTotal || p.Total != wantTotal || p.Lines[0].Description == "" {
		t.Fatal("incorrect preview")
	}
	if w := f.invoiceRequest("/confirm", data, "2", p.Token); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if w := f.invoiceRequest("", data, "3", ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	var count int
	var total int64
	if e := f.db.QueryRow("SELECT count(*) FROM transactions").Scan(&count); e != nil || count != 0 {
		t.Fatal("preview wrote transactions")
	}
	if w := f.invoiceRequest("/confirm", data, "1", p.Token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.invoiceRequest("/confirm", data, "1", p.Token); w.Code != 409 {
		t.Fatal("stale review accepted")
	}
	if again := preview("1"); again.New != 0 || again.Duplicate != wantCount {
		t.Fatal("retry did not reconcile")
	}
	if other := preview("2"); other.New != wantCount {
		t.Fatal("overlap across cards")
	}
	if e := f.db.QueryRow("SELECT count(*),sum(amount_cents) FROM card_installments l JOIN card_invoices i ON i.id=l.invoice_id WHERE i.card_id=1").Scan(&count, &total); e != nil || count != wantCount || total != wantTotal {
		t.Fatal("incorrect invoice total")
	}
	period := today()[:7]
	path := "/api/v1/cards/1/invoice"
	invoice := f.request("GET", path+"?month="+period, nil, "")
	var s statement
	if invoice.Code != 200 || json.Unmarshal(invoice.Body.Bytes(), &s) != nil || len(s.Lines) != wantCount || s.Due != wantTotal {
		t.Fatal("wrong statement")
	}
	if txs := f.request("GET", "/api/v1/transactions", nil, ""); !strings.Contains(txs.Body.String(), `"total":0`) {
		t.Fatal("purchase leaked into transactions")
	}
	pay := map[string]any{"month": period, "account_id": 1, "amount_cents": wantTotal}
	if w := f.request("POST", path+"/pay", pay, ""); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.request("POST", path+"/pay", pay, ""); w.Code != 409 {
		t.Fatal("duplicate payment")
	}
	txs := f.request("GET", "/api/v1/transactions", nil, "")
	if !strings.Contains(txs.Body.String(), "Fatura do cartão A "+period[5:]+"/"+period[:4]) || !strings.Contains(txs.Body.String(), `"total":1`) {
		t.Fatal(txs.Body.String())
	}
	var accounts []Account
	if w := f.request("GET", "/api/v1/accounts", nil, ""); json.Unmarshal(w.Body.Bytes(), &accounts) != nil || len(accounts) != 2 || accounts[0].Balance != 100000-wantTotal {
		t.Fatal("wrong balance")
	}
	if w := f.request("GET", path+"?month="+period, nil, ""); json.Unmarshal(w.Body.Bytes(), &s) != nil || s.Status != "paid" || s.Due != 0 {
		t.Fatal("invoice still open")
	}
}

func TestCSVFormats(t *testing.T) {
	p, e := expenseCSV([]byte("data,estabelecimento,valor\n2026-02-15,\"Loja, A\",12.34\n"), "2026")
	if e != nil || p.Total != 1234 {
		t.Fatal(p, e)
	}
	for _, s := range []string{"31/02;Loja;1,20", "12/02;Loja;12,345", "12/02;Loja;1,20\n;TOTAL;2,00", "12/02;Loja;1,20\n;TOTAL;1,20\n12/02;Loja;1,00"} {
		if _, e := expenseCSV([]byte("data;estabelecimento;valor\n"+s), "2026"); e == nil || !strings.Contains(e.Error(), "Linha") {
			t.Fatal(s, e)
		}
	}
}
