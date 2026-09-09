package main

import (
	"aigame/server/backend/infra"
	"aigame/server/backend/transport"
	"aigame/server/backend/usecase"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/alexedwards/argon2id"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if cmd != "serve" && cmd != "migrate" && cmd != "seed" {
		return errors.New("usage: backend [serve|migrate|seed USERNAME]")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p, e := infra.Open(ctx, os.Getenv("DATABASE_URL"), os.Getenv("TEST_DB_SCHEMA"))
	if e != nil {
		return errors.New("database configuration invalid")
	}
	defer p.Pool.Close()
	if cmd == "migrate" {
		if infra.Migrate(ctx, p) != nil {
			return errors.New("migration failed")
		}
		fmt.Println("migration complete")
		return nil
	}
	if cmd == "seed" {
		password := os.Getenv("G0_TEST_PASSWORD")
		if len(os.Args) != 3 || len(os.Args[2]) < 1 || len(os.Args[2]) > 64 || len(password) < 1 || len(password) > 256 {
			return errors.New("seed requires USERNAME and G0_TEST_PASSWORD (1..256 bytes)")
		}
		h, e := argon2id.CreateHash(password, argon2id.DefaultParams)
		if e != nil {
			return errors.New("password hashing failed")
		}
		b := make([]byte, 16)
		if _, e = rand.Read(b); e != nil {
			return errors.New("random generation failed")
		}
		b[6] = (b[6] & 15) | 64
		b[8] = (b[8] & 63) | 128
		id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
		_, e = p.Pool.Exec(ctx, "insert into players(player_id,username,password_hash) values($1,$2,$3) on conflict(username) do update set password_hash=excluded.password_hash", id, os.Args[2], h)
		if e != nil {
			return errors.New("seed failed")
		}
		fmt.Println("account provisioned")
		return nil
	}
	service := os.Getenv("ROOM_SERVICE_TOKEN")
	if len(service) < 32 {
		return errors.New("ROOM_SERVICE_TOKEN must contain at least 32 characters")
	}
	if v := os.Getenv("ROOM_ID"); v != "" && v != "g0-room" {
		return errors.New("ROOM_ID must be g0-room")
	}
	port, e := strconv.Atoi(env("ROOM_PORT", "7000"))
	if e != nil || port < 1 || port > 65535 {
		return errors.New("ROOM_PORT invalid")
	}
	cert, key := os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE")
	allow := os.Getenv("ALLOW_LOOPBACK_HTTP") == "1"
	if (cert == "") != (key == "") || cert == "" && !allow {
		return errors.New("TLS_CERT_FILE and TLS_KEY_FILE required unless ALLOW_LOOPBACK_HTTP=1")
	}
	s := &usecase.Service{Store: p, ServiceToken: service, Host: env("ROOM_HOST", "127.0.0.1"), Port: port}
	server := &http.Server{Addr: net.JoinHostPort(env("BACKEND_BIND_HOST", "127.0.0.1"), env("BACKEND_PORT", "8000")), Handler: transport.New(s, allow), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go func() {
		<-stop.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
	}()
	fmt.Println("G0 backend listening on", server.Addr)
	if cert != "" {
		e = server.ListenAndServeTLS(cert, key)
	} else {
		e = server.ListenAndServe()
	}
	if e != nil && !errors.Is(e, http.ErrServerClosed) {
		return errors.New("server failed")
	}
	return nil
}
func env(k, d string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return d
}
