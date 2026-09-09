package main

import (
	"aigame/server/backend/domain"
	"aigame/server/backend/infra"
	"aigame/server/backend/transport"
	"aigame/server/backend/usecase"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestG1Rooms(t *testing.T) {
	if os.Getenv("TEST_DB_INTEGRATION") != "1" {
		t.Skip("run via cmd/testdb")
	}
	ctx := context.Background()
	p, err := infra.Open(ctx, os.Getenv("DATABASE_URL"), os.Getenv("TEST_DB_SCHEMA"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Pool.Close()
	if err = infra.Migrate(ctx, p); err != nil {
		t.Fatal(err)
	}
	svc := &usecase.Service{Store: p, ServiceToken: "test-service"}
	post := func(path string, body any) int {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", path, bytes.NewReader(b))
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("Authorization", "Bearer test-service")
		rr := httptest.NewRecorder()
		transport.New(svc, true).ServeHTTP(rr, req)
		return rr.Code
	}
	t.Run("wire registration", func(t *testing.T) {
		body := map[string]any{"room_id": "wire", "host": "127.0.0.1", "port": 7000, "protocol_version": 1, "gameplay_content_hash": domain.Content, "resolved_config_hash": usecase.ConfigHash(), "capacity": 8, "generation": 1, "status": "ready"}
		if code := post("/v1/internal/rooms/register", body); code != 200 {
			t.Fatalf("registration=%d want 200", code)
		}
		body["extra"] = true
		if code := post("/v1/internal/rooms/register", body); code != 422 {
			t.Fatalf("unknown field=%d", code)
		}
	})
	// SQL setup deliberately isolates reservation and lease behavior from wire decoding.
	_, err = p.Pool.Exec(ctx, `delete from rooms`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Pool.Exec(ctx, `insert into rooms values('g1-room','127.0.0.1',7000,1,$1,$2,8,2,'ready',0,now())`, domain.Content, usecase.ConfigHash())
	if err != nil {
		t.Fatal(err)
	}
	var players []string
	for i := 0; i < 9; i++ {
		id := uuid.NewString()
		players = append(players, id)
		if _, err = p.Pool.Exec(ctx, "insert into players values($1,$2,'unused')", id, fmt.Sprintf("g1-%s", id)); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("capacity survives heartbeat", func(t *testing.T) {
		for i := 0; i < 8; i++ {
			if _, err := p.AllocateRoom(ctx, players[i], domain.Content, 1, usecase.ConfigHash()); err != nil {
				t.Fatal(err)
			}
		}
		if err := p.HeartbeatRoom(ctx, "g1-room", 2, 8, 0, "ready"); err != nil {
			t.Fatal(err)
		}
		if _, err := p.AllocateRoom(ctx, players[8], domain.Content, 1, usecase.ConfigHash()); err == nil {
			t.Error("ninth allocation accepted after heartbeat")
		}
	})
	t.Run("wrong generation unchanged", func(t *testing.T) {
		var before, after time.Time
		p.Pool.QueryRow(ctx, "select last_heartbeat from rooms where room_id='g1-room'").Scan(&before)
		if err := p.HeartbeatRoom(ctx, "g1-room", 1, 8, 0, "ready"); err == nil {
			t.Error("old generation accepted")
		}
		p.Pool.QueryRow(ctx, "select last_heartbeat from rooms where room_id='g1-room'").Scan(&after)
		if !before.Equal(after) {
			t.Error("lease changed")
		}
	})
	t.Run("wire heartbeat and ticket replay", func(t *testing.T) {
		body := map[string]any{"room_id": "g1-room", "generation": 2, "capacity": 8, "used_players": 2, "status": "ready"}
		if code := post("/v1/internal/rooms/heartbeat", body); code != 200 {
			t.Fatalf("heartbeat=%d", code)
		}
		body["generation"] = 1
		if code := post("/v1/internal/rooms/heartbeat", body); code != 401 {
			t.Fatalf("old heartbeat=%d", code)
		}
		session := usecase.Hash("g1-session")
		if err := p.CreateSession(ctx, players[0], session); err != nil {
			t.Fatal(err)
		}
		ticket := domain.Ticket{Token: "g1-ticket", PlayerID: players[0], RoomID: "g1-room", PresetID: domain.Preset, ConfigHash: usecase.ConfigHash(), Protocol: 1, Content: domain.Content}
		if err := p.IssueTicket(ctx, ticket, usecase.Hash(ticket.Token), session); err != nil {
			t.Fatal(err)
		}
		wrong := ticket
		wrong.Protocol = 2
		if _, err := svc.Redeem(ctx, wrong); err == nil {
			t.Fatal("wrong version accepted")
		}
		if _, err := svc.Redeem(ctx, ticket); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Redeem(ctx, ticket); err == nil {
			t.Fatal("replayed ticket accepted")
		}
		if err := p.ReleaseReservation(ctx, "g1-room", players[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := p.AllocateRoom(ctx, players[0], domain.Content, 1, usecase.ConfigHash()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("expired lease cannot revive", func(t *testing.T) {
		p.Pool.Exec(ctx, "update rooms set last_heartbeat=now()-interval '20 seconds',used_players=0")
		if err := p.HeartbeatRoom(ctx, "g1-room", 2, 8, 0, "ready"); err == nil {
			t.Error("expired heartbeat revived room")
		}
		if _, err := p.AllocateRoom(ctx, players[8], domain.Content, 1, usecase.ConfigHash()); err == nil {
			t.Error("expired room allocated")
		}
	})
}
