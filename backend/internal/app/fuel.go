package app

import (
	"database/sql"
	"encoding/json"
	"meu-financeiro/internal/domain"
	"net/http"
)

type forecastRoute struct {
	Name      string  `json:"name"`
	Distance  float64 `json:"distance"`
	Trips     int     `json:"trips"`
	RoundTrip bool    `json:"round_trip"`
}
type forecastInputs struct {
	Efficiency float64         `json:"efficiency"`
	Price      float64         `json:"price"`
	Routes     []forecastRoute `json:"routes"`
}

func (a *App) fuelForecast(w http.ResponseWriter, r *http.Request, u User) {
	month := r.PathValue("month")
	if !domain.Month(month) {
		fail(w, 400, "Mês inválido.")
		return
	}
	var inputs forecastInputs
	if r.Method == http.MethodGet {
		var data []byte
		err := a.DB.QueryRow("SELECT inputs FROM fuel_forecasts WHERE owner_id=? AND month=?", u.ID, month).Scan(&data)
		if err == sql.ErrNoRows {
			reply(w, 200, nil)
			return
		}
		if err != nil || json.Unmarshal(data, &inputs) != nil {
			fail(w, 500, "Falha ao ler a previsão.")
			return
		}
		reply(w, 200, inputs)
		return
	}
	if !decode(w, r, &inputs) {
		return
	}
	if inputs.Efficiency <= 0 || inputs.Efficiency > 1000 || inputs.Price <= 0 || inputs.Price > 1000 || len(inputs.Routes) == 0 || len(inputs.Routes) > 100 {
		fail(w, 400, "Confira o rendimento, o preço e os trajetos.")
		return
	}
	for _, route := range inputs.Routes {
		if !textValid(route.Name, 100) || route.Distance <= 0 || route.Distance > 100000 || route.Trips < 1 || route.Trips > 1000 {
			fail(w, 400, "Confira o nome, a distância e a frequência de cada trajeto.")
			return
		}
	}
	data, err := json.Marshal(inputs)
	if err == nil {
		_, err = a.DB.Exec("INSERT INTO fuel_forecasts(owner_id,month,inputs) VALUES(?,?,?) ON CONFLICT(owner_id,month) DO UPDATE SET inputs=excluded.inputs", u.ID, month, string(data))
	}
	if err != nil {
		fail(w, 500, "Falha ao salvar a previsão.")
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
