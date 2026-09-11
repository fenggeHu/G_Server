//go:build integration

package usecase

import (
	"aigame/server/backend/domain"
	"aigame/server/backend/infra"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
)

func openG3Store(t *testing.T) (*infra.PG, *Service, string) {
	t.Helper()
	if os.Getenv("TEST_DB_INTEGRATION") != "1" {
		t.Fatal("G3 tests must run through cmd/testdb with real PostgreSQL")
	}
	ctx := context.Background()
	p, err := infra.Open(ctx, os.Getenv("DATABASE_URL"), os.Getenv("TEST_DB_SCHEMA"))
	if err != nil {
		t.Fatal(err)
	}
	if err = infra.Migrate(ctx, p); err != nil {
		p.Pool.Close()
		t.Fatal(err)
	}
	player := uuid.NewString()
	if _, err = p.Pool.Exec(ctx, "insert into player(player_id,username,password_hash) values($1,$2,$3)", player, "g3-"+player, "unused"); err != nil {
		p.Pool.Close()
		t.Fatal(err)
	}
	return p, &Service{Store: p}, player
}

func TestG3ProgressRequiresSessionAndFencing(t *testing.T) {
	p, s, player := openG3Store(t)
	defer p.Pool.Close()
	ctx := context.Background()
	lease, err := s.AcquireSession(ctx, player, "room-a")
	if err != nil || lease.FencingToken == 0 {
		t.Fatalf("acquire session: %#v %v", lease, err)
	}
	payload, _ := json.Marshal(map[string]any{"item": "coin", "amount": 1})
	result, err := s.CommitProgress(ctx, domain.ProgressCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, OperationID: "pickup-1", Payload: payload})
	if err != nil || result.Status != domain.OperationSucceeded || result.Revision != 1 || result.Inventory["coin"] != 1 {
		t.Fatalf("commit progress: %#v %v", result, err)
	}
	if _, err = s.CommitProgress(ctx, domain.ProgressCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken - 1, OperationID: "pickup-2", Payload: payload}); err == nil {
		t.Fatal("stale fencing token must be rejected")
	}
	var amount int
	if err = p.Pool.QueryRow(ctx, "select (inventory->>'coin')::int from player_progress where player_id=$1", player).Scan(&amount); err != nil || amount != 1 {
		t.Fatalf("inventory=%d err=%v", amount, err)
	}
}

func TestG3ProgressSnapshotReturnsAuthoritativeState(t *testing.T) {
	p, s, player := openG3Store(t)
	defer p.Pool.Close()
	ctx := context.Background()
	lease, err := s.AcquireSession(ctx, player, "room-a")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"item":"coin","amount":1}`)
	if _, err = s.CommitProgress(ctx, domain.ProgressCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, OperationID: "snapshot-op", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.GetProgressSnapshot(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.PlayerID != player || snapshot.Revision != 1 || snapshot.Inventory["coin"] != 1 {
		t.Fatalf("unexpected progress snapshot: %#v", snapshot)
	}
	if snapshot.Equipment == nil || snapshot.Unlocks == nil {
		t.Fatalf("snapshot must include empty equipment/unlocks: %#v", snapshot)
	}
}

func TestG3EquipmentCommitUpdatesSnapshot(t *testing.T) {
	p, s, player := openG3Store(t)
	defer p.Pool.Close()
	ctx := context.Background()
	lease, err := s.AcquireSession(ctx, player, "room-a")
	if err != nil { t.Fatal(err) }
	equipped, err := s.CommitEquipment(ctx, domain.EquipmentCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, ExpectedRevision: 0, OperationID: "equip-1", Slot: "weapon", ItemID: "sword", Equipped: true})
	if err != nil || equipped.Equipment["weapon"] != "sword" || equipped.Revision != 1 { t.Fatalf("equip snapshot=%#v err=%v", equipped, err) }
	repeat, err := s.CommitEquipment(ctx, domain.EquipmentCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, ExpectedRevision: 0, OperationID: "equip-1", Slot: "weapon", ItemID: "sword", Equipped: true})
	if err != nil || repeat.Revision != 1 || repeat.Equipment["weapon"] != "sword" { t.Fatalf("idempotent equip=%#v err=%v", repeat, err) }
	unequipped, err := s.CommitEquipment(ctx, domain.EquipmentCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, ExpectedRevision: 1, OperationID: "unequip-1", Slot: "weapon", Equipped: false})
	if err != nil || len(unequipped.Equipment) != 0 || unequipped.Revision != 2 { t.Fatalf("unequip snapshot=%#v err=%v", unequipped, err) }
}

func TestG3ProgressOperationIsIdempotentAndRejectsPayloadChange(t *testing.T) {
	p, s, player := openG3Store(t)
	defer p.Pool.Close()
	ctx := context.Background()
	lease, err := s.AcquireSession(ctx, player, "room-a")
	if err != nil {
		t.Fatal(err)
	}
	if first.Inventory["coin"] != 1 {
		t.Fatalf("first commit must return inventory snapshot: %#v", first.Inventory)
	}
	one := []byte(`{"item":"coin","amount":1}`)
	first, err := s.CommitProgress(ctx, domain.ProgressCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, OperationID: "op", Payload: one})
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := s.CommitProgress(ctx, domain.ProgressCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, OperationID: "op", Payload: one})
	var firstPayload, repeatPayload map[string]any
	_ = json.Unmarshal(first.Payload, &firstPayload)
	_ = json.Unmarshal(repeat.Payload, &repeatPayload)
	if err != nil || repeat.Revision != first.Revision || firstPayload["item"] != repeatPayload["item"] || firstPayload["amount"] != repeatPayload["amount"] {
		t.Fatalf("repeat must return original result: %#v %#v %v", first, repeat, err)
	}
	if repeat.Inventory["coin"] != 1 {
		t.Fatalf("repeat must return inventory snapshot: %#v", repeat.Inventory)
	}
	if _, err = s.CommitProgress(ctx, domain.ProgressCommit{PlayerID: player, RoomID: "room-a", FencingToken: lease.FencingToken, OperationID: "op", Payload: []byte(`{"item":"gem","amount":1}`)}); err == nil {
		t.Fatal("different payload must be rejected")
	}
	queried, err := s.QueryProgress(ctx, domain.ProgressQuery{PlayerID: player, OperationID: "op"})
	if err != nil || queried.Revision != first.Revision {
		t.Fatalf("query committed operation: %#v %v", queried, err)
	}
}
