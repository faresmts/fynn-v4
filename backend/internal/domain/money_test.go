package domain

import "testing"

func TestDueDate(t *testing.T) {
	for _, x := range []struct {
		m    string
		d    int
		want string
	}{{"2026-02", 31, "2026-02-28"}, {"2024-02", 31, "2024-02-29"}, {"2026-04", 31, "2026-04-30"}, {"2026-12", 1, "2026-12-01"}} {
		got, e := DueDate(x.m, x.d)
		if e != nil || got != x.want {
			t.Fatal(got, e)
		}
	}
	if _, e := DueDate("invalid", 1); e == nil {
		t.Fatal("accepted invalid month")
	}
}
func TestAmountBounds(t *testing.T) {
	if !ValidAmount(1) || ValidAmount(0) || ValidAmount(-1) || ValidAmount(MaxCents+1) {
		t.Fatal("amount bounds")
	}
}
