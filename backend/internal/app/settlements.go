package app

import (
	"meu-financeiro/internal/domain"
	"net/http"
	"strconv"
)

type Settlement struct {
	ID          int64  `json:"id"`
	SenderID    int64  `json:"sender_id"`
	Sender      string `json:"sender"`
	ReceiverID  int64  `json:"receiver_id"`
	Receiver    string `json:"receiver"`
	AmountCents int64  `json:"amount_cents"`
	Month       string `json:"month"`
	SettledOn   string `json:"settled_on"`
	Note        string `json:"note"`
}

func (a *App) settlements(w http.ResponseWriter, r *http.Request, u User) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = today()[:7]
	}
	if !domain.Month(month) {
		fail(w, 400, "Competência inválida.")
		return
	}
	rows, e := a.DB.Query(`SELECT s.id,s.sender_id,su.name,s.receiver_id,ru.name,s.amount_cents,s.month,s.settled_on,s.note FROM settlements s JOIN users su ON su.id=s.sender_id JOIN users ru ON ru.id=s.receiver_id WHERE s.household_id=? AND s.month=? ORDER BY s.settled_on DESC,s.id DESC`, u.HouseholdID, month)
	if e != nil {
		fail(w, 500, "Não foi possível carregar os acertos.")
		return
	}
	defer rows.Close()
	out := []Settlement{}
	for rows.Next() {
		var x Settlement
		if e = rows.Scan(&x.ID, &x.SenderID, &x.Sender, &x.ReceiverID, &x.Receiver, &x.AmountCents, &x.Month, &x.SettledOn, &x.Note); e != nil {
			fail(w, 500, "Falha na leitura dos acertos.")
			return
		}
		out = append(out, x)
	}
	reply(w, 200, out)
}
func (a *App) createSettlement(w http.ResponseWriter, r *http.Request, u User) {
	var x struct {
		SenderID    int64  `json:"sender_id"`
		ReceiverID  int64  `json:"receiver_id"`
		AmountCents int64  `json:"amount_cents"`
		Month       string `json:"month"`
		SettledOn   string `json:"settled_on"`
		Note        string `json:"note"`
	}
	if !decode(w, r, &x) {
		return
	}
	if x.SenderID == x.ReceiverID || !domain.ValidAmount(x.AmountCents) || !domain.Month(x.Month) || !domain.Date(x.SettledOn) || x.SettledOn > today() || len(x.Note) > 200 {
		fail(w, 400, "Confira as pessoas, valor, competência e data do acerto.")
		return
	}
	var count int
	if e := a.DB.QueryRow("SELECT count(*) FROM users WHERE household_id=? AND id IN (?,?)", u.HouseholdID, x.SenderID, x.ReceiverID).Scan(&count); e != nil || count != 2 {
		fail(w, 400, "Os dois moradores precisam pertencer à mesma casa.")
		return
	}
	res, e := a.DB.Exec(`INSERT INTO settlements(household_id,sender_id,receiver_id,amount_cents,month,settled_on,note,created_by) VALUES(?,?,?,?,?,?,?,?)`, u.HouseholdID, x.SenderID, x.ReceiverID, x.AmountCents, x.Month, x.SettledOn, x.Note, u.ID)
	if e != nil {
		fail(w, 409, "Esse acerto já foi registrado ou não é válido.")
		return
	}
	id, _ := res.LastInsertId()
	a.DB.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'settlement',?,'create')", u.ID, id)
	reply(w, 201, map[string]int64{"id": id})
}
func (a *App) deleteSettlement(w http.ResponseWriter, r *http.Request, u User) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		fail(w, 404, "Acerto não encontrado.")
		return
	}
	res, e := a.DB.Exec("DELETE FROM settlements WHERE id=? AND household_id=?", id, u.HouseholdID)
	if e != nil || func() bool { n, _ := res.RowsAffected(); return n != 1 }() {
		fail(w, 404, "Acerto não encontrado.")
		return
	}
	a.DB.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'settlement',?,'delete')", u.ID, id)
	reply(w, 200, map[string]bool{"ok": true})
}
