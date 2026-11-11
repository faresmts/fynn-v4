package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/argon2"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID          int64  `json:"id"`
	HouseholdID int64  `json:"household_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	CSRF        string `json:"csrf,omitempty"`
}
type attempt struct {
	count int
	until time.Time
}
type App struct {
	DB         *sql.DB
	Origin     string
	Secure     bool
	Demo       bool
	mu         sync.Mutex
	attempts   map[string]attempt
	loginSlots chan struct{}
}

func New(db *sql.DB, origin string, secure, demo bool) *App {
	return &App{DB: db, Origin: origin, Secure: secure, Demo: demo, attempts: map[string]attempt{}, loginSlots: make(chan struct{}, 2)}
}
func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func digest(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func HashPassword(password string) string {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		panic(e)
	}
	h := argon2.IDKey([]byte(password), salt, 3, 32*1024, 1, 32)
	return base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(h)
}
func verifyPassword(password, hash string) bool {
	p := strings.Split(hash, ".")
	if len(p) != 2 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(p[0])
	if e != nil {
		return false
	}
	expected, e := base64.RawStdEncoding.DecodeString(p[1])
	if e != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 32*1024, 1, 32)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, "Dados inválidos. Confira os campos.")
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		fail(w, 400, "Envie um único objeto JSON.")
		return false
	}
	return true
}
func (a *App) acceptsOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	if origin == a.Origin {
		return true
	}
	configured, err := url.Parse(a.Origin)
	if err != nil || configured.Scheme != "http" || a.Secure {
		return false
	}
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]"} {
		if configured.Port() != "" {
			host += ":" + configured.Port()
		}
		if origin == "http://"+host {
			switch configured.Hostname() {
			case "localhost", "127.0.0.1", "::1":
				return true
			}
		}
	}
	return false
}

func (a *App) authenticated(next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("mf_session")
		if e != nil {
			fail(w, 401, "Entre para continuar.")
			return
		}
		var u User
		e = a.DB.QueryRow(`SELECT u.id,u.household_id,u.name,u.email,s.csrf FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=? AND s.expires_at>?`, digest(c.Value), time.Now().Unix()).Scan(&u.ID, &u.HouseholdID, &u.Name, &u.Email, &u.CSRF)
		if e != nil {
			fail(w, 401, "Sua sessão expirou. Entre novamente.")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if !a.acceptsOrigin(r.Header.Get("Origin")) || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(u.CSRF)) != 1 {
				fail(w, 403, "Não foi possível validar esta solicitação.")
				return
			}
		}
		next(w, r, u)
	}
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if !a.acceptsOrigin(r.Header.Get("Origin")) {
		fail(w, 403, "Origem não autorizada.")
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Password) > 1024 || len(input.Email) > 254 {
		fail(w, 400, "Dados inválidos.")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	a.mu.Lock()
	now := time.Now()
	for k, v := range a.attempts {
		if now.After(v.until) {
			delete(a.attempts, k)
		}
	}
	v := a.attempts[ip]
	if v.count >= 10 {
		a.mu.Unlock()
		fail(w, 429, "Muitas tentativas. Aguarde 15 minutos.")
		return
	}
	v.count++
	v.until = now.Add(15 * time.Minute)
	a.attempts[ip] = v
	a.mu.Unlock()
	select {
	case a.loginSlots <- struct{}{}:
		defer func() { <-a.loginSlots }()
	default:
		fail(w, 429, "Aguarde um instante e tente novamente.")
		return
	}
	var u User
	var hash string
	e := a.DB.QueryRow("SELECT id,household_id,name,email,password_hash FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(input.Email))).Scan(&u.ID, &u.HouseholdID, &u.Name, &u.Email, &hash)
	if e != nil {
		hash = HashPassword("unavailable-user")
	}
	if !verifyPassword(input.Password, hash) || e != nil {
		fail(w, 401, "E-mail ou senha incorretos.")
		return
	}
	session := token()
	u.CSRF = token()
	tx, e := a.DB.Begin()
	if e != nil {
		fail(w, 500, "Não foi possível entrar.")
		return
	}
	defer tx.Rollback()
	if c, err := r.Cookie("mf_session"); err == nil {
		if _, e = tx.Exec("DELETE FROM sessions WHERE token=?", digest(c.Value)); e != nil {
			fail(w, 500, "Não foi possível entrar.")
			return
		}
	}
	_, e = tx.Exec("DELETE FROM sessions WHERE expires_at<=?", now.Unix())
	if e == nil {
		_, e = tx.Exec("INSERT INTO sessions(token,user_id,csrf,expires_at) VALUES(?,?,?,?)", digest(session), u.ID, u.CSRF, now.Add(24*time.Hour).Unix())
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO audit(user_id,entity,entity_id,action) VALUES(?,'session',?,'login')", u.ID, u.ID)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		fail(w, 500, "Não foi possível entrar.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "mf_session", Value: session, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	a.mu.Lock()
	delete(a.attempts, ip)
	a.mu.Unlock()
	reply(w, 200, u)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request, u User) {
	c, _ := r.Cookie("mf_session")
	if _, e := a.DB.Exec("DELETE FROM sessions WHERE token=?", digest(c.Value)); e != nil {
		fail(w, 500, "Não foi possível sair.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "mf_session", Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	reply(w, 200, map[string]bool{"ok": true})
}

var errHouseholdFull = errors.New("a família já possui duas pessoas")

const registrationKeyDigest = "25e53b69d0314203d8c2e180702c566d94a69ca7e0fd26c4d37348c9e15a28cc"

func (a *App) registration(w http.ResponseWriter, r *http.Request) {
	if a.Demo || !a.acceptsOrigin(r.Header.Get("Origin")) {
		fail(w, 403, "Cadastro indisponível nesta origem.")
		return
	}
	var input struct{ Key, Name, Email, Password string }
	if !decode(w, r, &input) {
		return
	}
	if subtle.ConstantTimeCompare([]byte(digest(input.Key)), []byte(registrationKeyDigest)) != 1 {
		fail(w, 403, "Senha de autorização incorreta.")
		return
	}
	if r.URL.Path == "/api/v1/registration/unlock" {
		reply(w, 200, map[string]bool{"ok": true})
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 || input.Name == "" || len(input.Name) > 120 || len(input.Password) < 12 || len(input.Password) > 1024 {
		fail(w, 400, "Confira o nome, o e-mail e a senha de pelo menos 12 caracteres.")
		return
	}
	if err := Provision(a.DB, input.Name, input.Email, input.Password); err != nil {
		switch {
		case errors.Is(err, errHouseholdFull):
			fail(w, 409, "A casa já tem dois moradores cadastrados.")
		case strings.Contains(err.Error(), "UNIQUE constraint failed: users.email"):
			fail(w, 409, "Este e-mail já está cadastrado.")
		default:
			fail(w, 500, "Não foi possível criar a conta.")
		}
		return
	}
	reply(w, 201, map[string]bool{"ok": true})
}

func Provision(db *sql.DB, name, email, password string) error {
	if len(password) < 12 || len(password) > 1024 || !strings.Contains(email, "@") || len(name) == 0 {
		return errors.New("nome, e-mail e senha de pelo menos 12 caracteres são obrigatórios")
	}
	hash := HashPassword(password)
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT OR IGNORE INTO households(id,name) VALUES(1,'Nossa casa')"); e != nil {
		return e
	}
	var n int
	if e = tx.QueryRow("SELECT count(*) FROM users").Scan(&n); e != nil {
		return e
	}
	if n >= 2 {
		return errHouseholdFull
	}
	if _, e = tx.Exec("INSERT INTO users(household_id,name,email,password_hash) VALUES(1,?,?,?)", name, strings.ToLower(strings.TrimSpace(email)), hash); e != nil {
		return fmt.Errorf("provisionar usuário: %w", e)
	}
	for _, c := range []struct{ name, kind string }{{"Salário", "income"}, {"Trabalho extra", "income"}, {"Reembolso", "income"}, {"Outras receitas", "income"}, {"Moradia", "expense"}, {"Mercado", "expense"}, {"Alimentação", "expense"}, {"Transporte", "expense"}, {"Saúde", "expense"}, {"Lazer", "expense"}, {"Assinaturas", "expense"}, {"Outras despesas", "expense"}} {
		if _, e = tx.Exec("INSERT OR IGNORE INTO categories(household_id,name,type) VALUES(1,?,?)", c.name, c.kind); e != nil {
			return e
		}
	}
	return tx.Commit()
}
