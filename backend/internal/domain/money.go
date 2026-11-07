package domain

import (
	"errors"
	"time"
)

const MaxCents int64 = 100_000_000_000

func ValidAmount(v int64) bool { return v > 0 && v <= MaxCents }
func Date(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() >= 2000 && t.Year() <= 2100
}
func Month(s string) bool {
	t, e := time.Parse("2006-01", s)
	return e == nil && t.Year() >= 2000 && t.Year() <= 2100
}
func DueDate(month string, day int) (string, error) {
	if !Month(month) || day < 1 || day > 31 {
		return "", errors.New("período inválido")
	}
	t, _ := time.Parse("2006-01", month)
	last := t.AddDate(0, 1, -1).Day()
	if day > last {
		day = last
	}
	return time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), nil
}
