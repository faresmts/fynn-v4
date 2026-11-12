package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"meu-financeiro/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	t       *testing.T
	db      *sql.DB
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	db, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = Provision(db, "Alex", "alex@test.local", "strong-test-password"); e != nil {
		t.Fatal(e)
	}
	if e = Provision(db, "Marina", "marina@test.local", "other-test-password"); e != nil {
		t.Fatal(e)
	}
	f := &fixture{t: t, db: db, handler: New(db, "http://localhost", false, false).Handler(t.TempDir())}
	for _, q := range []string{`INSERT INTO accounts(id,household_id,owner_id,name,kind,opening_cents,opening_date) VALUES(1,1,1,'A','checking',100000,'2020-01-01'),(2,1,1,'B','savings',0,'2020-01-01'),(3,1,2,'Private','checking',999999,'2020-01-01')`} {
		if _, e = db.Exec(q); e != nil {
			t.Fatal(e)
		}
	}
	f.login("alex@test.local", "strong-test-password")
	return f
}
func (f *fixture) login(email, password string) {
	f.t.Helper()
	r := f.request("POST", "/api/v1/sessions", map[string]string{"email": email, "password": password}, "")
	if r.Code != 200 {
		f.t.Fatalf("login: %d %s", r.Code, r.Body.String())
	}
	f.cookie = r.Result().Cookies()[0]
	var u User
	json.Unmarshal(r.Body.Bytes(), &u)
	f.csrf = u.CSRF
}
func (f *fixture) request(method, path string, body any, key string) *httptest.ResponseRecorder {
	f.t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", f.csrf)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if f.cookie != nil {
		r.AddCookie(f.cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func expense() TransactionInput {
	c := int64(5)
	d := today()
	return TransactionInput{Type: "expense", Description: "Energy", Amount: 1, AccountID: 1, CategoryID: &c, Date: d, Competence: d[:7], DueDate: d, Status: "paid", PaidDate: &d, Visibility: "private"}
}
func (f *fixture) create(x TransactionInput, key string) int64 {
	f.t.Helper()
	w := f.request("POST", "/api/v1/transactions", x, key)
	if w.Code != 201 {
		f.t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	return out.ID
}
func TestCentAndIdempotency(t *testing.T) {
	f := setup(t)
	x := expense()
	id := f.create(x, "idempotent-key-001")
	w := f.request("POST", "/api/v1/transactions", x, "idempotent-key-001")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var n, b int64
	f.db.QueryRow("SELECT count(*) FROM transactions").Scan(&n)
	accounts, e := New(f.db, "", false, false).accountList(User{ID: 1, HouseholdID: 1})
	if e != nil {
		t.Fatal(e)
	}
	b = accounts[0].Balance
	if n != 1 || b != 99999 {
		t.Fatalf("n=%d balance=%d id=%d", n, b, id)
	}
	x.Amount = 2
	if w = f.request("POST", "/api/v1/transactions", x, "idempotent-key-001"); w.Code != 409 {
		t.Fatal(w.Code)
	}
}
func TestTransferExcludedFromConsumptionAndCash(t *testing.T) {
	f := setup(t)
	x := expense()
	dest := int64(2)
	x.Type = "transfer"
	x.CategoryID = nil
	x.DestinationID = &dest
	x.Amount = 10000
	f.create(x, "transfer-key-0001")
	a := New(f.db, "", false, false)
	accounts, e := a.accountList(User{ID: 1, HouseholdID: 1})
	if e != nil {
		t.Fatal(e)
	}
	if accounts[0].Balance != 90000 || accounts[1].Balance != 10000 {
		t.Fatal(accounts)
	}
	items, _ := a.transactionList(User{ID: 1, HouseholdID: 1}, "", "personal")
	sum := summarize(items, today()[:7])
	if sum.Income != 0 || sum.Expense != 0 || sum.Cash != 0 {
		t.Fatal(sum)
	}
}
func TestPaymentUpdatesOccurrenceAndPreservesExpected(t *testing.T) {
	f := setup(t)
	x := expense()
	x.Amount = 28000
	x.Status = "pending"
	x.PaidDate = nil
	id := f.create(x, "payment-key-00001")
	w := f.request("PATCH", "/api/v1/transactions/"+itoa(id), map[string]any{"version": 1, "action": "pay", "amount_cents": 31740, "paid_date": today()}, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var amount, expected, version int
	f.db.QueryRow("SELECT amount_cents,expected_cents,version FROM transactions WHERE id=?", id).Scan(&amount, &expected, &version)
	if amount != 31740 || expected != 28000 || version != 2 {
		t.Fatal(amount, expected, version)
	}
	w = f.request("PATCH", "/api/v1/transactions/"+itoa(id), map[string]any{"version": 1, "action": "cancel"}, "")
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
}
func itoa(v int64) string { b, _ := json.Marshal(v); return string(b) }
func TestPrivacyAcrossDetailListsReportsAndWrites(t *testing.T) {
	f := setup(t)
	x := expense()
	x.Description = "Secret medical expense"
	id := f.create(x, "private-key-00001")
	x.Visibility = "household"
	x.Description = "Shared expense"
	f.create(x, "shared-key-000001")
	f.login("marina@test.local", "other-test-password")
	for _, path := range []string{"/api/v1/transactions", "/api/v1/dashboard", "/api/v1/transactions?context=household", "/api/v1/dashboard?context=household"} {
		w := f.request("GET", path, nil, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), "Secret medical expense") {
			t.Fatalf("%s: %s", path, w.Body.String())
		}
		if strings.Contains(path, "household") && (!strings.Contains(w.Body.String(), "Shared expense") || strings.Contains(w.Body.String(), `"account":"A"`)) {
			t.Fatal(w.Body.String())
		}
	}
	w := f.request("PATCH", "/api/v1/transactions/"+itoa(id), map[string]any{"version": 1, "action": "cancel"}, "")
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = f.request("POST", "/api/v1/transactions", expense(), "attack-key-000001"); w.Code != 400 {
		t.Fatal(w.Code)
	}
}
func TestRecurrenceIsIdempotentAndClampsDay(t *testing.T) {
	f := setup(t)
	w := f.request("POST", "/api/v1/recurring-rules", Rule{Description: "Internet", Amount: 28000, Day: 31, AccountID: 1, CategoryID: 5, StartsOn: "2024-02"}, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		w = f.request("POST", "/api/v1/recurring-rules/generate", map[string]string{"month": "2024-02"}, "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	var n int
	var due string
	f.db.QueryRow("SELECT count(*),due_date FROM transactions").Scan(&n, &due)
	if n != 1 || due != "2024-02-29" {
		t.Fatal(n, due)
	}
}
func TestCancelReversesBalanceAndKeepsAudit(t *testing.T) {
	f := setup(t)
	id := f.create(expense(), "cancel-key-000001")
	w := f.request("PATCH", "/api/v1/transactions/"+itoa(id), map[string]any{"action": "cancel", "version": 1}, "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	a, _ := New(f.db, "", false, false).accountList(User{ID: 1, HouseholdID: 1})
	if a[0].Balance != 100000 {
		t.Fatal(a)
	}
	var n int
	f.db.QueryRow("SELECT count(*) FROM audit WHERE entity='transaction' AND entity_id=?", id).Scan(&n)
	if n != 2 {
		t.Fatal(n)
	}
}
func TestAuthCSRFAndLogout(t *testing.T) {
	f := setup(t)
	if !f.cookie.HttpOnly || f.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("insecure cookie")
	}
	csrf := f.csrf
	f.csrf = "wrong"
	if w := f.request("POST", "/api/v1/transactions", expense(), "csrf-key-00000001"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	f.csrf = csrf
	if w := f.request("DELETE", "/api/v1/sessions/current", nil, ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := f.request("GET", "/api/v1/dashboard", nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestFinancialValidation(t *testing.T) {
	f := setup(t)
	for _, mutate := range []func(*TransactionInput){func(x *TransactionInput) { x.Amount = 0 }, func(x *TransactionInput) { x.Amount = -1 }, func(x *TransactionInput) { x.Date = "2026-02-30" }, func(x *TransactionInput) { x.AccountID = 3 }, func(x *TransactionInput) { c := int64(1); x.CategoryID = &c }, func(x *TransactionInput) { d := time.Now().AddDate(0, 0, 2).Format("2006-01-02"); x.PaidDate = &d }} {
		x := expense()
		mutate(&x)
		w := f.request("POST", "/api/v1/transactions", x, "invalid-key-00001")
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	var n int
	f.db.QueryRow("SELECT count(*) FROM transactions").Scan(&n)
	if n != 0 {
		t.Fatal(n)
	}
}
func TestCashAndCompetenceAreIndependent(t *testing.T) {
	x := expense()
	paid := today()
	item := Transaction{Type: "expense", Amount: 20000, Competence: "2020-01", Status: "paid", PaidDate: &paid}
	current := summarize([]Transaction{item}, x.Competence)
	old := summarize([]Transaction{item}, "2020-01")
	if current.Cash != -20000 || current.Expense != 0 || old.Expense != 20000 || old.Cash != 0 {
		t.Fatal(current, old)
	}
}
func TestLoginRateLimit(t *testing.T) {
	f := setup(t)
	for i := 0; i < 11; i++ {
		w := f.request("POST", "/api/v1/sessions", map[string]string{"email": "alex@test.local", "password": "wrong"}, "")
		if i == 10 && w.Code != 429 {
			t.Fatal(w.Code)
		}
	}
}

func TestPersonalRentShares(t *testing.T) {
	f := setup(t)
	x := expense()
	x.Visibility, x.Amount = "household", 179258
	f.create(x, "shared-rent-key-001")
	for i, income := range []int{643500, 275858} {
		if _, err := f.db.Exec("INSERT INTO income_snapshots(household_id,user_id,month,income_cents,rule_mode,confirmed) VALUES(1,?,?,?,'monthly_income_ratio',1)", i+1, x.Competence, income); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range []int64{125470, 53788} {
		if i == 1 {
			f.login("marina@test.local", "other-test-password")
		}
		w := f.request("GET", "/api/v1/transactions?context=personal&month="+x.Competence, nil, "")
		var response struct{ Items []Transaction }
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || len(response.Items) != 1 {
			t.Fatal(w.Body.String())
		}
		row := response.Items[0]
		if row.Amount != x.Amount || row.PersonalShare == nil || *row.PersonalShare != want {
			t.Fatalf("unexpected rent share for resident %d: %+v", i+1, row)
		}
	}
}
