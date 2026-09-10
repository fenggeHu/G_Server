package main

import (
	"aigame/server/backend/infra"
	"aigame/server/backend/transport"
	"aigame/server/backend/usecase"
	"bytes"
	"context"
	"encoding/json"
	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestG0Integration(t *testing.T) {
	if os.Getenv("TEST_DB_INTEGRATION") != "1" {
		t.Skip("run via go run ./cmd/testdb")
	}
	ctx := context.Background()
	p, e := infra.Open(ctx, os.Getenv("DATABASE_URL"), os.Getenv("TEST_DB_SCHEMA"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Pool.Close()
	if e = infra.Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	pw := "integration-password"
	hash, _ := argon2id.CreateHash(pw, argon2id.DefaultParams)
	id := uuid.New()
	if _, e = p.Pool.Exec(ctx, "insert into player(player_id,username,password_hash) values($1,$2,$3)", id, "integration", hash); e != nil {
		t.Fatal(e)
	}
	svc := &usecase.Service{Store: p, ServiceToken: os.Getenv("ROOM_SERVICE_TOKEN"), Host: "127.0.0.1", Port: 7000}
	h := transport.New(svc, true)
	server := httptest.NewServer(h)
	defer server.Close()
	client := server.Client()
	post := func(path string, v any, auth string) *http.Response {
		b, _ := json.Marshal(v)
		r, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(b))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		x, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		return x
	}
	login := post("/v1/auth/login", map[string]string{"username": "integration", "password": pw}, "")
	if login.StatusCode != 200 {
		t.Fatalf("login %d", login.StatusCode)
	}
	var ld struct {
		Token string `json:"access_token"`
	}
	json.NewDecoder(login.Body).Decode(&ld)
	login.Body.Close()
	ticket := post("/v1/tickets", map[string]any{"preset_id": "exploration", "protocol_version": 1, "gameplay_content_hash": "g0-empty-v1"}, "Bearer "+ld.Token)
	if ticket.StatusCode != 200 {
		t.Fatalf("ticket %d", ticket.StatusCode)
	}
	var td map[string]any
	json.NewDecoder(ticket.Body).Decode(&td)
	ticket.Body.Close()
	redeem := map[string]any{"ticket": td["ticket"], "room_id": "g0-room", "protocol_version": 1, "gameplay_content_hash": "g0-empty-v1", "resolved_config_hash": td["resolved_config_hash"]}
	wrong := post("/v1/internal/tickets/redeem", redeem, "Bearer forged")
	if wrong.StatusCode != 401 {
		t.Fatalf("forged service %d", wrong.StatusCode)
	}
	var wg sync.WaitGroup
	results := make(chan int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := post("/v1/internal/tickets/redeem", redeem, "Bearer "+svc.ServiceToken)
			results <- r.StatusCode
			r.Body.Close()
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	var statuses []int
	for status := range results {
		statuses = append(statuses, status)
		if status == 200 {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("redeem successes=%d statuses=%v", ok, statuses)
	}
	logout := post("/v1/auth/logout", nil, "Bearer "+ld.Token)
	if logout.StatusCode != 200 {
		t.Fatalf("logout %d", logout.StatusCode)
	}
	expired, _ := p.Pool.Exec(ctx, "update session set expires_at=now()-interval '1 second'")
	_ = expired
	bad := post("/v1/auth/login", map[string]any{"username": "integration", "password": pw, "extra": "secret"}, "")
	if bad.StatusCode != 422 {
		t.Fatalf("strict=%d", bad.StatusCode)
	}
	big := post("/v1/auth/login", strings.Repeat("x", 9000), "")
	if big.StatusCode != 413 {
		t.Fatalf("body=%d", big.StatusCode)
	}
	forgedHeader := httptest.NewRequest("GET", "/health/live", nil)
	forgedHeader.Header.Set("X-Forwarded-Proto", "https")
	forgedHeader.RemoteAddr = "192.0.2.1:1"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, forgedHeader)
	if rr.Code != 400 {
		t.Fatalf("proxy tls=%d", rr.Code)
	}
}
