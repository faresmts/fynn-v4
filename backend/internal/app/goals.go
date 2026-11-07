package app

import (
	"meu-financeiro/internal/domain"
	"net/http"
)

type Goal struct {
	ID          int64   `json:"id"`
	OwnerID     int64   `json:"owner_id"`
	Owner       string  `json:"owner"`
	Name        string  `json:"name"`
	TargetCents int64   `json:"target_cents"`
	SavedCents  int64   `json:"saved_cents"`
	Deadline    *string `json:"deadline"`
	Visibility  string  `json:"visibility"`
	ProgressBps int64   `json:"progress_bps"`
}

func (a *App) goals(w http.ResponseWriter, r *http.Request, u User) {
	rows, e := a.DB.Query(`SELECT g.id,g.owner_id,u.name,g.name,g.target_cents,COALESCE((SELECT SUM(c.amount_cents) FROM goal_contributions c WHERE c.goal_id=g.id),0),g.deadline,g.visibility FROM goals g JOIN users u ON u.id=g.owner_id WHERE g.household_id=? AND g.archived=0 AND (g.owner_id=? OR g.visibility='household') ORDER BY g.deadline IS NULL,g.deadline,g.id`, u.HouseholdID, u.ID)
	if e != nil {
		fail(w, 500, "Não foi possível carregar as metas.")
		return
	}
	defer rows.Close()
	out := []Goal{}
	for rows.Next() {
		var x Goal
		if e = rows.Scan(&x.ID, &x.OwnerID, &x.Owner, &x.Name, &x.TargetCents, &x.SavedCents, &x.Deadline, &x.Visibility); e != nil {
			fail(w, 500, "Falha na leitura das metas.")
			return
		}
		if x.TargetCents > 0 {
			x.ProgressBps = x.SavedCents * 10000 / x.TargetCents
		}
		out = append(out, x)
	}
	reply(w, 200, out)
}
func (a *App) createGoal(w http.ResponseWriter, r *http.Request, u User) {
	var x struct {
		Name        string  `json:"name"`
		TargetCents int64   `json:"target_cents"`
		Deadline    *string `json:"deadline"`
		Visibility  string  `json:"visibility"`
	}
	if !decode(w, r, &x) {
		return
	}
	if !textValid(x.Name, 100) || !domain.ValidAmount(x.TargetCents) || (x.Visibility != "private" && x.Visibility != "household") || (x.Deadline != nil && !domain.Date(*x.Deadline)) {
		fail(w, 400, "Confira o nome, alvo, prazo e visibilidade da meta.")
		return
	}
	res, e := a.DB.Exec("INSERT INTO goals(household_id,owner_id,name,target_cents,deadline,visibility) VALUES(?,?,?,?,?,?)", u.HouseholdID, u.ID, x.Name, x.TargetCents, x.Deadline, x.Visibility)
	if e != nil {
		fail(w, 500, "Não foi possível criar a meta.")
		return
	}
	id, _ := res.LastInsertId()
	reply(w, 201, map[string]int64{"id": id})
}
func (a *App) contributeGoal(w http.ResponseWriter, r *http.Request, u User) {
	var x struct {
		AmountCents int64  `json:"amount_cents"`
		Date        string `json:"date"`
		Note        string `json:"note"`
	}
	if !decode(w, r, &x) {
		return
	}
	if !domain.ValidAmount(x.AmountCents) || !domain.Date(x.Date) || x.Date > today() || len(x.Note) > 200 {
		fail(w, 400, "Confira o valor e a data do aporte.")
		return
	}
	id := r.PathValue("id")
	var owner, household int64
	if e := a.DB.QueryRow("SELECT owner_id,household_id FROM goals WHERE id=? AND archived=0", id).Scan(&owner, &household); e != nil || household != u.HouseholdID || (owner != u.ID) {
		fail(w, 404, "Meta não encontrada ou sem permissão.")
		return
	}
	res, e := a.DB.Exec("INSERT INTO goal_contributions(goal_id,user_id,amount_cents,contributed_on,note) VALUES(?,?,?,?,?)", id, u.ID, x.AmountCents, x.Date, x.Note)
	if e != nil {
		fail(w, 500, "Não foi possível registrar o aporte.")
		return
	}
	newID, _ := res.LastInsertId()
	reply(w, 201, map[string]int64{"id": newID})
}
