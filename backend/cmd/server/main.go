package main

import (
	"context"
	"flag"
	"log"
	"meu-financeiro/internal/app"
	"meu-financeiro/internal/store"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)

func env(k, d string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return d
}
func main() {
	demo := flag.Bool("demo", false, "Criar dados fictícios; permitido somente em loopback e banco vazio")
	provision := flag.Bool("provision", false, "Criar usuário a partir de MF_NAME, MF_EMAIL, MF_PASSWORD")
	flag.Parse()
	db, e := store.Open(env("MF_DATABASE", "../data/financeiro.db"))
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	if *provision {
		if e = app.Provision(db, os.Getenv("MF_NAME"), os.Getenv("MF_EMAIL"), os.Getenv("MF_PASSWORD")); e != nil {
			log.Fatal(e)
		}
		log.Print("Usuário criado.")
		return
	}
	addr := env("MF_ADDR", "127.0.0.1:8080")
	origin := env("MF_ORIGIN", "http://127.0.0.1:8080")
	secure := strings.HasPrefix(origin, "https://")
	host, _, e := net.SplitHostPort(addr)
	if e != nil {
		log.Fatal(e)
	}
	ip := net.ParseIP(host)
	loopback := ip != nil && ip.IsLoopback()
	if !secure && !loopback {
		log.Fatal("HTTP sem TLS só pode escutar em loopback")
	}
	if *demo {
		if !loopback || secure {
			log.Fatal("Demonstração permitida somente em loopback HTTP")
		}
		if e = app.SeedDemo(db); e != nil {
			log.Fatal(e)
		}
	} else {
		var demoUsers int
		if e = db.QueryRow("SELECT count(*) FROM users WHERE email IN ('alex@demo.local','marina@demo.local')").Scan(&demoUsers); e != nil {
			log.Fatal(e)
		}
		if demoUsers > 0 {
			log.Fatal("Banco de demonstração exige --demo; use um banco separado para dados reais")
		}
	}
	a := app.New(db, origin, secure, *demo)
	server := &http.Server{Addr: addr, Handler: a.Handler(env("MF_FRONTEND", "../frontend/dist")), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	log.Printf("Meu Financeiro em %s", origin)
	if e = server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
