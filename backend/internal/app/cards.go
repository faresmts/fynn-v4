package app

import (
	"net/http"
	"strings"
)

type Card struct {
	ID          int64  `json:"id"`
	Institution string `json:"institution"`
	Name        string `json:"name"`
	Last4       string `json:"last4,omitempty"`
	Active      bool   `json:"active"`
}

func (a *App) cards(w http.ResponseWriter, r *http.Request, u User) {
	rows, e := a.DB.Query(`SELECT id,institution,name,COALESCE(last4,''),active FROM cards WHERE owner_id=? AND household_id=? ORDER BY active DESC,name`, u.ID, u.HouseholdID)
	if e != nil {
		fail(w, 500, "Não foi possível carregar os cartões.")
		return
	}
	defer rows.Close()
	out := []Card{}
	for rows.Next() {
		var c Card
		var active int
		if rows.Scan(&c.ID, &c.Institution, &c.Name, &c.Last4, &active) == nil {
			c.Active = active == 1
			out = append(out, c)
		}
	}
	reply(w, 200, out)
}
func (a *App) createCard(w http.ResponseWriter, r *http.Request, u User) {
	var in struct {
		Institution string `json:"institution"`
		Name        string `json:"name"`
		Last4       string `json:"last4"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Institution = strings.ToLower(strings.TrimSpace(in.Institution))
	in.Name = strings.TrimSpace(in.Name)
	in.Last4 = strings.TrimSpace(in.Last4)
	if in.Institution != "picpay" && in.Institution != "nubank" && in.Institution != "mercado_pago" {
		fail(w, 400, "Instituição de cartão inválida.")
		return
	}
	if !textValid(in.Name, 80) || len(in.Last4) > 4 {
		fail(w, 400, "Informe o nome e os 4 últimos dígitos do cartão.")
		return
	}
	res, e := a.DB.Exec(`INSERT INTO cards(household_id,owner_id,institution,name,last4) VALUES(?,?,?,?,?)`, u.HouseholdID, u.ID, in.Institution, in.Name, in.Last4)
	if e != nil {
		fail(w, 409, "Esse cartão já está cadastrado.")
		return
	}
	id, _ := res.LastInsertId()
	reply(w, 201, Card{ID: id, Institution: in.Institution, Name: in.Name, Last4: in.Last4, Active: true})
}
