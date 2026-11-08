package app

import (
	"encoding/json"
	"testing"
)

func TestHouseholdInvoiceSplit(t *testing.T) {
	f := setup(t)
	month := today()[:7]
	if _, err := f.db.Exec("INSERT INTO cards(id,household_id,owner_id,institution,name) VALUES(1,1,1,'picpay','A')"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("INSERT INTO card_invoices(id,card_id,reference_month,due_date,total_cents) VALUES(1,1,?,?,10001)", month, today()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("INSERT INTO card_installments(id,invoice_id,description,purchase_date,amount_cents) VALUES(1,1,'Mercado',?,10001)", today()); err != nil {
		t.Fatal(err)
	}
	bill := expense()
	bill.Visibility = "household"
	bill.Status = "pending"
	bill.PaidDate = nil
	bill.Amount = 2000
	ordinaryID := f.create(bill, "ordinary-household-bill")
	share := "/api/v1/invoice-lines/1/share"
	split := "/api/v1/invoice-lines/1/split"
	for i := 0; i < 2; i++ {
		if w := f.request("POST", share, nil, ""); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	var statementView statement
	w := f.request("GET", "/api/v1/cards/1/invoice?month="+month, nil, "")
	if json.Unmarshal(w.Body.Bytes(), &statementView) != nil || len(statementView.Lines) != 1 || !statementView.Lines[0].Shared || statementView.Due != 10001 {
		t.Fatal("incorrect invoice")
	}
	var count int
	if err := f.db.QueryRow("SELECT count(*) FROM transactions").Scan(&count); err != nil || count != 1 {
		t.Fatal("sharing posted a transaction")
	}
	f.login("marina@test.local", "other-test-password")
	if w := f.request("POST", share, nil, ""); w.Code != 404 {
		t.Fatal("non-owner shared a card purchase")
	}
	custom := map[string]any{"version": 1, "mode": "custom", "custom_cents": map[string]int{"1": 7001, "2": 3000}}
	if w := f.request("POST", split, custom, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := f.request("POST", split, custom, ""); w.Code != 409 {
		t.Fatal("stale split accepted")
	}
	if w := f.request("POST", "/api/v1/transactions/"+itoa(ordinaryID)+"/split", map[string]any{"version": 1, "mode": "equal"}, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	totals := func(paid int64) {
		t.Helper()
		w := f.request("GET", "/api/v1/dashboard?context=household&month="+month, nil, "")
		var v struct {
			Costs        []HouseholdExpense `json:"household_expenses"`
			Contributors []ContributorTotal `json:"contributors"`
			Totals       Totals             `json:"totals"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || len(v.Costs) != 2 || len(v.Contributors) != 2 || v.Totals.Expense != 12001 {
			t.Fatal(w.Body.String())
		}
		if v.Contributors[0].PaidCents != paid || v.Contributors[0].ResponsibilityCents != 8001 || v.Contributors[1].ResponsibilityCents != 4000 {
			t.Fatal(v.Contributors)
		}
	}
	totals(0)
	f.login("alex@test.local", "strong-test-password")
	if w := f.request("POST", "/api/v1/cards/1/invoice/pay", map[string]any{"month": month, "account_id": 1, "amount_cents": 10001}, ""); w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	totals(10001)
}
