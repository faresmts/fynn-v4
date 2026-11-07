package app

import (
	"net/http"
	"os"
	"path/filepath"
)

func (a *App) Handler(frontend string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if e := a.DB.Ping(); e != nil {
			fail(w, 503, "Indisponível")
			return
		}
		reply(w, 200, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /api/v1/config", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]bool{"demo": a.Demo}) })
	m.HandleFunc("POST /api/v1/sessions", a.login)
	m.HandleFunc("DELETE /api/v1/sessions/current", a.authenticated(a.logout))
	m.HandleFunc("GET /api/v1/me", a.authenticated(func(w http.ResponseWriter, r *http.Request, u User) { reply(w, 200, u) }))
	m.HandleFunc("GET /api/v1/accounts", a.authenticated(a.accounts))
	m.HandleFunc("POST /api/v1/accounts", a.authenticated(a.createAccount))
	m.HandleFunc("GET /api/v1/categories", a.authenticated(a.categories))
	m.HandleFunc("POST /api/v1/categories", a.authenticated(a.createCategory))
	m.HandleFunc("GET /api/v1/transactions", a.authenticated(a.transactions))
	m.HandleFunc("POST /api/v1/transactions", a.authenticated(a.createTransaction))
	m.HandleFunc("PATCH /api/v1/transactions/{id}", a.authenticated(a.updateTransaction))
	m.HandleFunc("GET /api/v1/dashboard", a.authenticated(a.dashboard))
	m.HandleFunc("GET /api/v1/recurring-rules", a.authenticated(a.rules))
	m.HandleFunc("POST /api/v1/recurring-rules", a.authenticated(a.createRule))
	m.HandleFunc("POST /api/v1/recurring-rules/generate", a.authenticated(a.generateRules))
	m.HandleFunc("PATCH /api/v1/settings", a.authenticated(a.settings))
	m.HandleFunc("GET /api/v1/sharing/income-snapshots", a.authenticated(a.incomeSnapshots))
	m.HandleFunc("POST /api/v1/sharing/income-snapshots", a.authenticated(a.saveIncomeSnapshot))
	m.HandleFunc("GET /api/v1/sharing/summary", a.authenticated(a.sharingSummary))
	m.HandleFunc("POST /api/v1/transactions/{id}/split", a.authenticated(a.splitTransaction))
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "Recurso não encontrado.") })
	fs := http.FileServer(http.Dir(frontend))
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			fail(w, 405, "Método não permitido.")
			return
		}
		if _, e := os.Stat(filepath.Join(frontend, filepath.Clean("/"+r.URL.Path))); e != nil {
			http.ServeFile(w, r, filepath.Join(frontend, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("Cache-Control", "no-store")
		m.ServeHTTP(w, r)
	})
}
