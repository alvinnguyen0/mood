package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"mood-tracker/internal/store"
	"mood-tracker/internal/web"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "file:mood.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}
	st, err := store.OpenSQLite(dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer st.Close()

	srv, err := web.New(st)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	// Background session cleanup
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := st.DeleteExpiredSessions(context.Background()); err != nil {
					log.Printf("session cleanup: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("mood tracker listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
