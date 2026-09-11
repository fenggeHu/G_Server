//go:build integration

package main

import (
	"aigame/server/backend/domain"
	"aigame/server/backend/infra"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestProgressSchemaVersionPolicy(t *testing.T) {
	if os.Getenv("TEST_DB_INTEGRATION") != "1" {
		t.Skip("run via go run ./cmd/testdb")
	}
	ctx := context.Background()
	p, e := infra.Open(ctx, os.Getenv("DATABASE_URL"), os.Getenv("TEST_DB_SCHEMA")+"_progress")
	if e != nil {
		t.Fatal(e)
	}
	defer p.Pool.Close()
	if e = infra.Migrate(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e = infra.MigrateProgressSchema(ctx, p); e != nil {
		t.Fatal(e)
	}

	pid := uuid.New()
	if _, e = p.Pool.Exec(ctx, "insert into player(player_id,username,password_hash) values($1,$2,$3)", pid, "schema-"+pid.String(), "x"); e != nil {
		t.Fatal(e)
	}

	// 新行通过会话获取即标记为当前版本。
	if _, e = p.AcquireSession(ctx, pid.String(), "schema-room"); e != nil {
		t.Fatal(e)
	}
	var version int
	if e = p.Pool.QueryRow(ctx, "select schema_version from player_progress where player_id=$1", pid).Scan(&version); e != nil {
		t.Fatal(e)
	}
	if version != infra.ProgressSchemaVersion {
		t.Fatalf("new row version=%d want=%d", version, infra.ProgressSchemaVersion)
	}

	// 旧版本行被显式迁移升级。
	if _, e = p.Pool.Exec(ctx, "update player_progress set schema_version=1 where player_id=$1", pid); e != nil {
		t.Fatal(e)
	}
	if e = infra.MigrateProgressSchema(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e = p.Pool.QueryRow(ctx, "select schema_version from player_progress where player_id=$1", pid).Scan(&version); e != nil {
		t.Fatal(e)
	}
	if version != infra.ProgressSchemaVersion {
		t.Fatalf("migrated version=%d want=%d", version, infra.ProgressSchemaVersion)
	}

	// 高于支持的版本：迁移与各读取路径都必须拒绝。
	if _, e = p.Pool.Exec(ctx, "update player_progress set schema_version=99 where player_id=$1", pid); e != nil {
		t.Fatal(e)
	}
	if e = infra.MigrateProgressSchema(ctx, p); !errors.Is(e, infra.ErrProgressSchemaUnsupported) {
		t.Fatalf("expected unsupported migration, got %v", e)
	}
	if _, e = p.GetProgressSnapshot(ctx, pid.String()); !errors.Is(e, infra.ErrProgressSchemaUnsupported) {
		t.Fatalf("expected unsupported snapshot read, got %v", e)
	}
	if _, e = p.GetQuestSnapshot(ctx, pid.String()); !errors.Is(e, infra.ErrProgressSchemaUnsupported) {
		t.Fatalf("expected unsupported quest snapshot read, got %v", e)
	}
	checks := []func() error{
		func() error {
			_, err := p.CommitEquipment(ctx, domain.EquipmentCommit{PlayerID: pid.String(), RoomID: "schema-room", FencingToken: 1, OperationID: "future-equipment", Slot: "hand", ItemID: "coin"})
			return err
		},
		func() error {
			_, err := p.CommitQuest(ctx, domain.QuestCommit{PlayerID: pid.String(), RoomID: "schema-room", FencingToken: 1, OperationID: "future-quest", QuestID: "starter_collect", ObjectiveID: "collect_coin", Required: 2, Amount: 1})
			return err
		},
		func() error {
			_, err := p.CommitProgress(ctx, domain.ProgressCommit{PlayerID: pid.String(), RoomID: "schema-room", FencingToken: 1, OperationID: "future-pickup", Payload: []byte(`{"item":"coin","amount":1}`)})
			return err
		},
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, infra.ErrProgressSchemaUnsupported) {
			t.Fatalf("write %d: expected unsupported version, got %v", i, err)
		}
	}
	var unchanged bool
	if e = p.Pool.QueryRow(ctx, `select revision=0 and inventory='{}'::jsonb and equipment='{}'::jsonb and quests='[]'::jsonb and not exists(select 1 from progress_operation where player_id=$1) from player_progress where player_id=$1`, pid).Scan(&unchanged); e != nil || !unchanged {
		t.Fatalf("rejected writes changed progress: %v", e)
	}
	t.Log("PROGRESS_SCHEMA_POLICY_PASS")
}
