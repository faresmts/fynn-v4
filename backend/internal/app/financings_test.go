package app

import "testing"

func TestFinancingPayments(t *testing.T) {
	f := setup(t)
	contract := financing{Name: "Carro", Total: 10000, Paid: 5000, Installments: 4, Installment: 3500}
	if w := f.request("POST", "/api/v1/financings", contract, ""); w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	pay := func(expected int64, status int) {
		t.Helper()
		w := f.request("POST", "/api/v1/financings/1/pay", map[string]int64{"paid_cents": expected}, "")
		if w.Code != status {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	pay(5000, 200)
	pay(5000, 409)
	pay(8500, 200)
	pay(10000, 409)
	var paid int64
	if err := f.db.QueryRow("SELECT paid_cents FROM financings WHERE id=1").Scan(&paid); err != nil || paid != 10000 {
		t.Fatal(paid, err)
	}
	var n int
	if err := f.db.QueryRow("SELECT count(*) FROM financing_payments WHERE paid_on=?", today()).Scan(&n); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	for i, status := range []int{200, 404} {
		if w := f.request("DELETE", "/api/v1/financings/1/payments/2", nil, ""); w.Code != status {
			t.Fatal(i, w.Code)
		}
	}
	if err := f.db.QueryRow("SELECT paid_cents FROM financings WHERE id=1").Scan(&paid); err != nil || paid != 8500 {
		t.Fatal(paid, err)
	}
	pay(8500, 200)
}
