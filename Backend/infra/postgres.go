package infra

import (
	"aigame/server/backend/domain"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProgressSchemaVersion 是当前服务器支持的 player_progress 数据版本。
const ProgressSchemaVersion = 2

// ErrProgressSchemaUnsupported 表示库中存在高于服务器支持的进度版本，
// 旧服务器不得静默处理新数据。
var ErrProgressSchemaUnsupported = errors.New("unsupported progress schema version")

// requireProgressVersion 校验单行 player_progress.schema_version 是否受支持。
func requireProgressVersion(version int) error {
	if version > ProgressSchemaVersion {
		return fmt.Errorf("%w: found %d, supported %d", ErrProgressSchemaUnsupported, version, ProgressSchemaVersion)
	}
	return nil
}

type PG struct {
	Pool   *pgxpool.Pool
	Schema string
}

func Open(ctx context.Context, url, schema string) (*PG, error) {
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, e
	}
	if schema != "" {
		for _, r := range schema {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
				return nil, fmt.Errorf("invalid database schema")
			}
		}
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
	}
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, e
	}
	return &PG{p, schema}, nil
}
func (p *PG) q(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.Pool.QueryRow(ctx, sql, args...)
}
func pgxIdent(s string) string {
	if s == "" {
		return "public"
	}
	return pgx.Identifier{s}.Sanitize()
}
func (p *PG) FindPlayer(c context.Context, u string) (domain.Player, error) {
	var x domain.Player
	e := p.q(c, "select player_id,password_hash from player where username=$1", u).Scan(&x.ID, &x.PasswordHash)
	return x, e
}
func (p *PG) CreateSession(c context.Context, id, h string) error {
	_, e := p.Pool.Exec(c, "insert into session(player_id,token_hash,expires_at) values($1,$2,now()+interval '15 minutes')", id, h)
	return e
}
func (p *PG) FindSession(c context.Context, h string) (domain.Session, error) {
	var x domain.Session
	e := p.q(c, "select player_id,token_hash from session where token_hash=$1 and revoked_at is null and expires_at>now()", h).Scan(&x.PlayerID, &x.TokenHash)
	return x, e
}
func (p *PG) RevokeSession(c context.Context, h string) error {
	_, e := p.Pool.Exec(c, "update session set revoked_at=now() where token_hash=$1", h)
	return e
}
func (p *PG) IssueTicket(c context.Context, x domain.Ticket, ticketHash, sessionHash string) error {
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return e
	}
	defer tx.Rollback(c)
	var id string
	if e = tx.QueryRow(c, "select player_id from session where token_hash=$1 and player_id=$2 and revoked_at is null and expires_at>now() for update", sessionHash, x.PlayerID).Scan(&id); e != nil {
		return e
	}
	_, e = tx.Exec(c, "insert into ticket(token_hash,player_id,session_hash,expires_at,room_id,preset_id,resolved_config_hash,protocol_version,gameplay_content_hash) values($1,$2,$3,now()+interval '60 seconds',$4,$5,$6,$7,$8)", ticketHash, id, sessionHash, x.RoomID, x.PresetID, x.ConfigHash, x.Protocol, x.Content)
	if e != nil {
		return e
	}
	return tx.Commit(c)
}
func (p *PG) RedeemTicket(c context.Context, x domain.Ticket) (domain.Ticket, error) {
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return x, e
	}
	defer tx.Rollback(c)
	var out domain.Ticket
	var session string
	if e = tx.QueryRow(c, "select s.token_hash from session s join ticket t on t.session_hash=s.token_hash where t.token_hash=$1 for update of s", domainHash(x.Token)).Scan(&session); e != nil {
		return x, e
	}
	e = tx.QueryRow(c, "update ticket set redeemed_at=now() where token_hash=$1 and redeemed_at is null and expires_at>now() and room_id=$2 and protocol_version=$3 and gameplay_content_hash=$4 and resolved_config_hash=$5 and preset_id=$6 and exists(select 1 from session s where s.token_hash=ticket.session_hash and s.player_id=ticket.player_id and s.revoked_at is null and s.expires_at>now()) returning player_id,room_id,preset_id,resolved_config_hash", domainHash(x.Token), x.RoomID, x.Protocol, x.Content, x.ConfigHash, x.PresetID).Scan(&out.PlayerID, &out.RoomID, &out.PresetID, &out.ConfigHash)
	if e != nil {
		return x, e
	}
	out.Token = x.Token
	if e = tx.Commit(c); e != nil {
		return x, e
	}
	return out, nil
}
func domainHash(s string) string           { b := sha256.Sum256([]byte(s)); return fmt.Sprintf("%x", b) }
func (p *PG) Ping(c context.Context) error { return p.Pool.Ping(c) }
func (p *PG) AcquireSession(c context.Context, player, room string) (domain.SessionLease, error) {
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return domain.SessionLease{}, e
	}
	defer tx.Rollback(c)
	if _, e = tx.Exec(c, `insert into player_progress(player_id,schema_version) values($1,$2) on conflict(player_id) do nothing`, player, ProgressSchemaVersion); e != nil {
		return domain.SessionLease{}, e
	}
	var x domain.SessionLease
	e = tx.QueryRow(c, `insert into active_player_session(player_id,fencing_token,room_id,lease_expires_at) values($1,1,$2,now()+interval '30 seconds') on conflict(player_id) do update set fencing_token=active_player_session.fencing_token+1,room_id=excluded.room_id,lease_expires_at=excluded.lease_expires_at where active_player_session.lease_expires_at<=now() returning player_id,room_id,fencing_token,lease_expires_at`, player, room).Scan(&x.PlayerID, &x.RoomID, &x.FencingToken, &x.LeaseExpiresAt)
	if e != nil {
		return x, e
	}
	return x, tx.Commit(c)
}
func (p *PG) RenewSession(c context.Context, x domain.SessionLease) (domain.SessionLease, error) {
	var out = x
	e := p.Pool.QueryRow(c, `update active_player_session set lease_expires_at=now()+interval '30 seconds' where player_id=$1 and room_id=$2 and fencing_token=$3 and lease_expires_at>now() returning lease_expires_at`, x.PlayerID, x.RoomID, x.FencingToken).Scan(&out.LeaseExpiresAt)
	return out, e
}
func (p *PG) ReleaseSession(c context.Context, x domain.SessionLease) error {
	_, e := p.Pool.Exec(c, `delete from active_player_session where player_id=$1 and room_id=$2 and fencing_token=$3`, x.PlayerID, x.RoomID, x.FencingToken)
	return e
}
func (p *PG) QueryProgress(c context.Context, q domain.ProgressQuery) (domain.OperationResult, error) {
	var x domain.OperationResult
	var raw []byte
	e := p.Pool.QueryRow(c, `select operation_id,status,result,revision from progress_operation where player_id=$1 and operation_id=$2`, q.PlayerID, q.OperationID).Scan(&x.OperationID, &x.Status, &raw, &x.Revision)
	x.Payload = raw
	return x, e
}

func (p *PG) GetProgressSnapshot(c context.Context, playerID string) (domain.ProgressSnapshot, error) {
	var snapshot domain.ProgressSnapshot
	var inventoryRaw, equipmentRaw, unlocksRaw, questsRaw []byte
	err := p.Pool.QueryRow(c, `select player_id,schema_version,revision,inventory,equipment,unlocks,quests from player_progress where player_id=$1`, playerID).Scan(&snapshot.PlayerID, &snapshot.SchemaVersion, &snapshot.Revision, &inventoryRaw, &equipmentRaw, &unlocksRaw, &questsRaw)
	if err != nil {
		return snapshot, err
	}
	if err = requireProgressVersion(snapshot.SchemaVersion); err != nil {
		return snapshot, err
	}
	if err = json.Unmarshal(inventoryRaw, &snapshot.Inventory); err != nil {
		return snapshot, err
	}
	if err = json.Unmarshal(equipmentRaw, &snapshot.Equipment); err != nil {
		return snapshot, err
	}
	if err = json.Unmarshal(unlocksRaw, &snapshot.Unlocks); err != nil {
		return snapshot, err
	}
	if err = json.Unmarshal(questsRaw, &snapshot.Quests); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (p *PG) GetQuestSnapshot(c context.Context, playerID string) (domain.QuestSnapshot, error) {
	var snapshot domain.QuestSnapshot
	var raw []byte
	var version int
	err := p.Pool.QueryRow(c, `select player_id,schema_version,revision,unlocks from player_progress where player_id=$1`, playerID).Scan(&snapshot.PlayerID, &version, &snapshot.Revision, &raw)
	if err != nil {
		return snapshot, err
	}
	if err = requireProgressVersion(version); err != nil {
		return snapshot, err
	}
	if err = json.Unmarshal(raw, &snapshot.Unlocks); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (p *PG) CommitQuest(c context.Context, in domain.QuestCommit) (domain.ProgressSnapshot, error) {
	if in.PlayerID == "" || in.RoomID == "" || in.QuestID == "" || in.ObjectiveID == "" || in.Required < 1 || in.Amount < 1 || in.Amount > in.Required || in.OperationID == "" || in.FencingToken < 1 {
		return domain.ProgressSnapshot{}, fmt.Errorf("invalid quest request")
	}
	tx, err := p.Pool.Begin(c)
	if err != nil {
		return domain.ProgressSnapshot{}, err
	}
	defer tx.Rollback(c)
	var existingRaw []byte
	var existingRevision int64
	err = tx.QueryRow(c, `select result,revision from progress_operation where player_id=$1 and operation_id=$2`, in.PlayerID, in.OperationID).Scan(&existingRaw, &existingRevision)
	if err == nil {
		var existing map[string]any
		if json.Unmarshal(existingRaw, &existing) != nil {
			return domain.ProgressSnapshot{}, fmt.Errorf("quest operation conflict")
		}
		amount, ok := existing["amount"].(float64)
		if !ok || existing["quest_id"] != in.QuestID || existing["objective_id"] != in.ObjectiveID || int(amount) != in.Amount {
			return domain.ProgressSnapshot{}, fmt.Errorf("quest operation conflict")
		}
		if err = tx.Commit(c); err != nil {
			return domain.ProgressSnapshot{}, err
		}
		return p.GetProgressSnapshot(c, in.PlayerID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.ProgressSnapshot{}, err
	}
	var revision int64
	var questsRaw, inventoryRaw []byte
	if err = tx.QueryRow(c, `select revision,quests,inventory from player_progress where player_id=$1 for update`, in.PlayerID).Scan(&revision, &questsRaw, &inventoryRaw); err != nil {
		return domain.ProgressSnapshot{}, err
	}
	var quests []domain.QuestState
	if err = json.Unmarshal(questsRaw, &quests); err != nil {
		return domain.ProgressSnapshot{}, err
	}
	var inventory map[string]int
	if err = json.Unmarshal(inventoryRaw, &inventory); err != nil {
		return domain.ProgressSnapshot{}, err
	}
	if inventory == nil {
		inventory = map[string]int{}
	}
	questIndex := -1
	for i := range quests {
		if quests[i].QuestID == in.QuestID {
			questIndex = i
			break
		}
	}
	if questIndex < 0 {
		quests = append(quests, domain.QuestState{QuestID: in.QuestID})
		questIndex = len(quests) - 1
	}
	wasCompleted := quests[questIndex].Completed
	objectiveIndex := -1
	for i := range quests[questIndex].Objectives {
		if quests[questIndex].Objectives[i].ObjectiveID == in.ObjectiveID {
			objectiveIndex = i
			break
		}
	}
	if objectiveIndex < 0 {
		quests[questIndex].Objectives = append(quests[questIndex].Objectives, domain.QuestObjective{ObjectiveID: in.ObjectiveID, Required: in.Required})
		objectiveIndex = len(quests[questIndex].Objectives) - 1
	}
	if quests[questIndex].Objectives[objectiveIndex].Required != in.Required {
		return domain.ProgressSnapshot{}, fmt.Errorf("quest requirement mismatch")
	}
	objective := &quests[questIndex].Objectives[objectiveIndex]
	objective.Progress += in.Amount
	if objective.Progress > objective.Required {
		objective.Progress = objective.Required
	}
	quests[questIndex].Completed = true
	for _, candidate := range quests[questIndex].Objectives {
		if candidate.Progress < candidate.Required {
			quests[questIndex].Completed = false
			break
		}
	}
	quests[questIndex].Revision = revision + 1
	// 完成瞬间在同一事务内发放服务器配置的奖励，只发一次。
	if !wasCompleted && quests[questIndex].Completed {
		for itemID, amount := range in.Reward {
			inventory[itemID] += amount
		}
	}
	updatedQuests, _ := json.Marshal(quests)
	updatedInventory, _ := json.Marshal(inventory)
	var newRevision int64
	if err = tx.QueryRow(c, `update player_progress p set revision=p.revision+1,quests=$5,inventory=$6 from active_player_session s where p.player_id=$1 and s.player_id=p.player_id and s.room_id=$2 and s.fencing_token=$3 and s.lease_expires_at>now() and p.revision=$4 returning p.revision`, in.PlayerID, in.RoomID, in.FencingToken, in.ExpectedRevision, updatedQuests, updatedInventory).Scan(&newRevision); err != nil {
		return domain.ProgressSnapshot{}, err
	}
	payload, _ := json.Marshal(in)
	if _, err = tx.Exec(c, `insert into progress_operation(player_id,operation_id,payload_hash,status,result,revision) values($1,$2,$3,'succeeded',$4,$5)`, in.PlayerID, in.OperationID, domainHash(string(payload)), payload, newRevision); err != nil {
		return domain.ProgressSnapshot{}, err
	}
	if err = tx.Commit(c); err != nil {
		return domain.ProgressSnapshot{}, err
	}
	return p.GetProgressSnapshot(c, in.PlayerID)
}

func (p *PG) CommitEquipment(c context.Context, in domain.EquipmentCommit) (domain.ProgressSnapshot, error) {
	if in.PlayerID == "" || in.RoomID == "" || in.Slot == "" || len(in.Slot) > 32 || len(in.ItemID) > 128 || in.OperationID == "" || in.FencingToken < 1 {
		return domain.ProgressSnapshot{}, fmt.Errorf("invalid equipment request")
	}
	tx, err := p.Pool.Begin(c)
	if err != nil { return domain.ProgressSnapshot{}, err }
	defer tx.Rollback(c)
	var existingPayload []byte
	var existingRevision int64
	err = tx.QueryRow(c, `select result,revision from progress_operation where player_id=$1 and operation_id=$2`, in.PlayerID, in.OperationID).Scan(&existingPayload, &existingRevision)
	if err == nil {
		var existing map[string]any
		if json.Unmarshal(existingPayload, &existing) != nil || existing["slot"] != in.Slot || existing["item_id"] != in.ItemID || existing["equipped"] != in.Equipped {
			return domain.ProgressSnapshot{}, fmt.Errorf("equipment operation conflict")
		}
		var out domain.ProgressSnapshot
		var inventoryRaw, equipmentRaw, unlocksRaw []byte
		if err = tx.QueryRow(c, `select player_id,schema_version,revision,inventory,equipment,unlocks from player_progress where player_id=$1`, in.PlayerID).Scan(&out.PlayerID, &out.SchemaVersion, &out.Revision, &inventoryRaw, &equipmentRaw, &unlocksRaw); err != nil { return out, err }
		if err = requireProgressVersion(out.SchemaVersion); err != nil { return out, err }
		if err = json.Unmarshal(inventoryRaw, &out.Inventory); err != nil { return out, err }
		if err = json.Unmarshal(equipmentRaw, &out.Equipment); err != nil { return out, err }
		if err = json.Unmarshal(unlocksRaw, &out.Unlocks); err != nil { return out, err }
		return out, tx.Commit(c)
	}
	if err != pgx.ErrNoRows { return domain.ProgressSnapshot{}, err }
	var revision int64
	var equipmentRaw, inventoryRaw, unlocksRaw []byte
	var owned bool
	if in.Equipped {
		err = tx.QueryRow(c, `select coalesce((inventory->>$2)::int,0)>0 from player_progress where player_id=$1 for update`, in.PlayerID, in.ItemID).Scan(&owned)
	} else {
		err = tx.QueryRow(c, `select coalesce(equipment->>$2,'')=$3 from player_progress where player_id=$1 for update`, in.PlayerID, in.Slot, in.ItemID).Scan(&owned)
	}
	if err != nil { return domain.ProgressSnapshot{}, err }
	if !owned { return domain.ProgressSnapshot{}, domain.ErrConflict }
	if in.Equipped {
		err = tx.QueryRow(c, `update player_progress p set revision=p.revision+1,equipment=jsonb_set(p.equipment,array[$5],to_jsonb($6::text),true) from active_player_session s where p.player_id=$1 and s.player_id=p.player_id and s.room_id=$2 and s.fencing_token=$3 and s.lease_expires_at>now() and p.revision=$4 returning p.revision,p.equipment,p.inventory,p.unlocks`, in.PlayerID,in.RoomID,in.FencingToken,in.ExpectedRevision,in.Slot,in.ItemID).Scan(&revision,&equipmentRaw,&inventoryRaw,&unlocksRaw)
	} else {
		err = tx.QueryRow(c, `update player_progress p set revision=p.revision+1,equipment=p.equipment-$5 from active_player_session s where p.player_id=$1 and s.player_id=p.player_id and s.room_id=$2 and s.fencing_token=$3 and s.lease_expires_at>now() and p.revision=$4 returning p.revision,p.equipment,p.inventory,p.unlocks`, in.PlayerID,in.RoomID,in.FencingToken,in.ExpectedRevision,in.Slot).Scan(&revision,&equipmentRaw,&inventoryRaw,&unlocksRaw)
	}
	if err != nil { return domain.ProgressSnapshot{}, err }
	var out domain.ProgressSnapshot
	out.PlayerID, out.Revision = in.PlayerID, revision
	if err=json.Unmarshal(equipmentRaw,&out.Equipment); err != nil { return out,err }
	if err=json.Unmarshal(inventoryRaw,&out.Inventory); err != nil { return out,err }
	if err=json.Unmarshal(unlocksRaw,&out.Unlocks); err != nil { return out,err }
	operationPayload, _ := json.Marshal(in)
	if _, err = tx.Exec(c, `insert into progress_operation(player_id,operation_id,payload_hash,status,result,revision) values($1,$2,$3,'succeeded',$4,$5)`, in.PlayerID, in.OperationID, domainHash(string(operationPayload)), operationPayload, revision); err != nil { return out,err }
	if err=tx.Commit(c); err != nil { return out,err }
	return out,nil
}
func (p *PG) CommitProgress(c context.Context, in domain.ProgressCommit) (domain.OperationResult, error) {
	h := domainHash(string(in.Payload))
	var pickup struct {
		Item   string `json:"item"`
		Amount int    `json:"amount"`
	}
	if e := json.Unmarshal(in.Payload, &pickup); e != nil || pickup.Item == "" || len(pickup.Item) > 64 || pickup.Amount != 1 {
		return domain.OperationResult{}, fmt.Errorf("invalid progress payload")
	}
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return domain.OperationResult{}, e
	}
	defer tx.Rollback(c)
	var old domain.OperationResult
	var oldHash string
	var raw []byte
	e = tx.QueryRow(c, `select status,result,revision,payload_hash from progress_operation where player_id=$1 and operation_id=$2`, in.PlayerID, in.OperationID).Scan(&old.Status, &raw, &old.Revision, &oldHash)
	if e == nil {
		if oldHash != h {
			return old, errors.New("operation payload conflict")
		}
		old.OperationID = in.OperationID
		old.Payload = raw
		if e = tx.QueryRow(c, `select inventory from player_progress where player_id=$1`, in.PlayerID).Scan(&raw); e != nil {
			return old, e
		}
		if e = json.Unmarshal(raw, &old.Inventory); e != nil {
			return old, e
		}
		return old, tx.Commit(c)
	}
	if e != pgx.ErrNoRows {
		return old, e
	}
	var revision int64
	e = tx.QueryRow(c, `update player_progress p set revision=p.revision+1, inventory=jsonb_set(p.inventory, array[$5], to_jsonb(coalesce((p.inventory->>$5)::int,0)+$6), true) from active_player_session s where p.player_id=$1 and s.player_id=p.player_id and s.room_id=$2 and s.fencing_token=$3 and s.lease_expires_at>now() and p.revision=$4 returning p.revision`, in.PlayerID, in.RoomID, in.FencingToken, in.ExpectedRevision, pickup.Item, pickup.Amount).Scan(&revision)
	if e != nil {
		return old, e
	}
	_, e = tx.Exec(c, `insert into progress_operation(player_id,operation_id,payload_hash,status,result,revision) values($1,$2,$3,'succeeded',$4,$5)`, in.PlayerID, in.OperationID, h, in.Payload, revision)
	if e != nil {
		return old, e
	}
	var inventory map[string]int
	if e = tx.QueryRow(c, `select inventory from player_progress where player_id=$1`, in.PlayerID).Scan(&raw); e != nil {
		return old, e
	}
	if e = json.Unmarshal(raw, &inventory); e != nil {
		return old, e
	}
	if e = tx.Commit(c); e != nil {
		return old, e
	}
	return domain.OperationResult{OperationID: in.OperationID, Status: domain.OperationSucceeded, Revision: revision, Payload: in.Payload, Inventory: inventory}, nil
}
func (p *PG) RegisterRoom(c context.Context, x domain.RoomRecord) (domain.RoomRecord, error) {
    e := p.Pool.QueryRow(c, `insert into room(room_id,host,port,protocol_version,gameplay_content_hash,resolved_config_hash,capacity,generation,status,used_players,last_heartbeat) values($1,$2,$3,$4,$5,$6,$7,$8,$9,0,now()) on conflict(room_id) do update set host=excluded.host,port=excluded.port,protocol_version=excluded.protocol_version,gameplay_content_hash=excluded.gameplay_content_hash,resolved_config_hash=excluded.resolved_config_hash,capacity=excluded.capacity,generation=excluded.generation,status=excluded.status,last_heartbeat=now() where room.generation<=excluded.generation returning room_id,host,port,protocol_version,gameplay_content_hash,resolved_config_hash,status,generation,capacity,used_players,last_heartbeat`, x.ID, x.Host, x.Port, x.ProtocolVersion, x.GameplayContentHash, x.ResolvedConfigHash, x.Capacity, x.Generation, x.Status).Scan(&x.ID, &x.Host, &x.Port, &x.ProtocolVersion, &x.GameplayContentHash, &x.ResolvedConfigHash, &x.Status, &x.Generation, &x.Capacity, &x.UsedPlayers, &x.LastHeartbeat)
	return x, e
}
func (p *PG) HeartbeatRoom(c context.Context, id string, g, cap, used int, status string) error {
	if cap < 1 || cap > 8 || used < 0 || used > cap || (status != "ready" && status != "draining") {
		return fmt.Errorf("invalid heartbeat")
	}
	r, e := p.Pool.Exec(c, `update room set status=$4,last_heartbeat=now() where room_id=$1 and generation=$2 and capacity=$3 and last_heartbeat>now()-interval '15 seconds'`, id, g, cap, status)
	if e != nil {
		return e
	}
	if r.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
func (p *PG) AllocateRoom(c context.Context, player, content string, proto int, config string) (domain.RoomRecord, error) {
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return domain.RoomRecord{}, e
	}
	defer tx.Rollback(c)
	var r domain.RoomRecord
	e = tx.QueryRow(c, `select room_id,host,port,protocol_version,gameplay_content_hash,resolved_config_hash,status,generation,capacity,used_players,last_heartbeat from room where status='ready' and last_heartbeat>now()-interval '15 seconds' and used_players<capacity and protocol_version=$1 and gameplay_content_hash=$2 and resolved_config_hash=$3 order by room_id for update skip locked limit 1`, proto, content, config).Scan(&r.ID, &r.Host, &r.Port, &r.ProtocolVersion, &r.GameplayContentHash, &r.ResolvedConfigHash, &r.Status, &r.Generation, &r.Capacity, &r.UsedPlayers, &r.LastHeartbeat)
	if e != nil {
		return r, e
	}
	var releasedRoom string
	e = tx.QueryRow(c, `update room_reservation set released_at=now(),status=0 where player_id=$1 and released_at is null returning room_id`, player).Scan(&releasedRoom)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return r, e
	}
	if e == nil {
		if _, e = tx.Exec(c, `update room set used_players=greatest(used_players-1,0) where room_id=$1`, releasedRoom); e != nil {
			return r, e
		}
	}
	var reserved string
	if e = tx.QueryRow(c, `insert into room_reservation(room_id,player_id) values($1,$2) on conflict(room_id,player_id) do update set released_at=null,created_at=now(),status=1 returning player_id`, r.ID, player).Scan(&reserved); e != nil {
		return r, e
	}
	_, e = tx.Exec(c, `update room set used_players=used_players+1 where room_id=$1`, r.ID)
	if e != nil {
		return r, e
	}
	e = tx.Commit(c)
	return r, e
}
func (p *PG) ReleaseReservation(c context.Context, room, player string) error {
	tx, e := p.Pool.Begin(c)
	if e != nil {
		return e
	}
	defer tx.Rollback(c)
	var changed bool
	e = tx.QueryRow(c, `update room_reservation set released_at=now() where room_id=$1 and player_id=$2 and released_at is null returning true`, room, player).Scan(&changed)
	if e == pgx.ErrNoRows {
		return tx.Commit(c)
	}
	if e != nil {
		return e
	}
	if _, e = tx.Exec(c, `update room set used_players=greatest(used_players-1,0) where room_id=$1`, room); e != nil {
		return e
	}
	return tx.Commit(c)
}
func migrationsDir() string {
	path := os.Getenv("MIGRATIONS_DIR")
	if path == "" {
		path = "migrations"
	}
	return path
}
func Migrate(ctx context.Context, p *PG) error {
	path := migrationsDir()
	entries, e := os.ReadDir(path)
	if e != nil {
		return e
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".sql") && !strings.HasSuffix(name, ".down.sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	tx, e := p.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if p.Schema != "" {
		if _, e = tx.Exec(ctx, "create schema if not exists "+pgxIdent(p.Schema)); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, "select pg_advisory_xact_lock(884422)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "create table if not exists schema_migration(name text primary key,checksum text not null,created_at timestamptz not null default now(),updated_at timestamptz not null default now(),status smallint not null default 1)"); e != nil {
		return e
	}
	for _, name := range files {
		b, e := os.ReadFile(filepath.Join(path, name))
		if e != nil {
			return e
		}
		checksum := domainHash(string(b))
		var old string
		e = tx.QueryRow(ctx, "select checksum from schema_migration where name=$1", name).Scan(&old)
		if e == nil {
			if old != checksum {
				return fmt.Errorf("migration checksum mismatch: %s", name)
			}
			continue
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if _, e = tx.Exec(ctx, string(b)); e != nil {
			return fmt.Errorf("migration %s: %w", name, e)
		}
		if _, e = tx.Exec(ctx, "insert into schema_migration(name,checksum) values($1,$2)", name, checksum); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// Rollback reverts the most recently applied migrations in reverse order using
// the matching <name>.down.sql file. steps <= 0 rolls back all applied
// migrations. It returns the migration names that were rolled back.
func Rollback(ctx context.Context, p *PG, steps int) ([]string, error) {
	path := migrationsDir()
	tx, e := p.Pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if p.Schema != "" {
		if _, e = tx.Exec(ctx, "create schema if not exists "+pgxIdent(p.Schema)); e != nil {
			return nil, e
		}
	}
	if _, e = tx.Exec(ctx, "select pg_advisory_xact_lock(884422)"); e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, "create table if not exists schema_migration(name text primary key,checksum text not null,created_at timestamptz not null default now(),updated_at timestamptz not null default now(),status smallint not null default 1)"); e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, "select name from schema_migration order by name desc")
	if e != nil {
		return nil, e
	}
	var applied []string
	for rows.Next() {
		var name string
		if e = rows.Scan(&name); e != nil {
			rows.Close()
			return nil, e
		}
		applied = append(applied, name)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return nil, e
	}
	var rolled []string
	for _, name := range applied {
		if steps > 0 && len(rolled) >= steps {
			break
		}
		down := strings.TrimSuffix(name, ".sql") + ".down.sql"
		b, e := os.ReadFile(filepath.Join(path, down))
		if e != nil {
			return rolled, fmt.Errorf("missing down migration %s: %w", down, e)
		}
		if _, e = tx.Exec(ctx, string(b)); e != nil {
			return rolled, fmt.Errorf("down migration %s: %w", down, e)
		}
		if _, e = tx.Exec(ctx, "delete from schema_migration where name=$1", name); e != nil {
			return rolled, e
		}
		rolled = append(rolled, name)
	}
	if e = tx.Commit(ctx); e != nil {
		return rolled, e
	}
	return rolled, nil
}

// MigrateProgressSchema 将所有 player_progress 行显式升级到当前数据版本。
// 仅做版本标记迁移（v1 -> v2 为新增 quests 列，已有列默认值补齐）；
// 若发现高于服务器支持的版本则返回 ErrProgressSchemaUnsupported，
// 防止旧服务器静默处理新数据。
func MigrateProgressSchema(ctx context.Context, p *PG) error {
	var maxVersion int
	if e := p.Pool.QueryRow(ctx, "select coalesce(max(schema_version),0) from player_progress").Scan(&maxVersion); e != nil {
		return e
	}
	if maxVersion > ProgressSchemaVersion {
		return fmt.Errorf("%w: found %d, supported %d", ErrProgressSchemaUnsupported, maxVersion, ProgressSchemaVersion)
	}
	if _, e := p.Pool.Exec(ctx, "update player_progress set schema_version=$1 where schema_version < $1", ProgressSchemaVersion); e != nil {
		return e
	}
	return nil
}
