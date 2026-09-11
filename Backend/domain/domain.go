package domain

import (
	"errors"
	"time"
)

var ErrConflict = errors.New("conflict")

const (
	Preset              = "exploration"
	Content             = "g0-empty-v1"
	CombatPreset        = "coop_combat"
	CombatContent       = "g4-coop-combat-v1"
	Room                = "g0-room"
	Proto               = 1
	DefaultMap          = "starter_valley"
	DefaultMapContent   = "starter_valley-0"
	DefaultMapAuthority = "starter_valley-authority-0"
)

type Player struct {
	ID           string
	PasswordHash string
}
type Session struct{ PlayerID, TokenHash string }
type Ticket struct {
	Token, PlayerID, RoomID, PresetID, ConfigHash string
	MapID, MapContentVersion, MapAuthorityVersion string
	Protocol                                      int
	Content                                       string
}

type RoomRecord struct {
	ID                  string    `json:"room_id"`
	Host                string    `json:"host"`
	ProtocolVersion     int       `json:"protocol_version"`
	GameplayContentHash string    `json:"gameplay_content_hash"`
	ResolvedConfigHash  string    `json:"resolved_config_hash"`
	Status              string    `json:"status"`
	Port                int       `json:"port"`
	Generation          int       `json:"generation"`
	Capacity            int       `json:"capacity"`
	UsedPlayers         int       `json:"used_players"`
	LastHeartbeat       time.Time `json:"last_heartbeat"`
	MapID               string    `json:"map_id,omitempty"`
	MapContentVersion   string    `json:"map_content_version,omitempty"`
	MapAuthorityVersion string    `json:"map_authority_version,omitempty"`
}

type Reservation struct{ RoomID, PlayerID string }

type PlayerProgress struct {
	SchemaVersion int               `json:"schema_version"`
	PlayerID      string            `json:"player_id"`
	Revision      int64             `json:"revision"`
	Inventory     map[string]int    `json:"inventory"`
	Equipment     map[string]string `json:"equipment"`
	Unlocks       []string          `json:"unlocks"`
}

type ProgressSnapshot struct {
	PlayerID      string            `json:"player_id"`
	SchemaVersion int               `json:"schema_version"`
	Revision      int64             `json:"revision"`
	Inventory     map[string]int    `json:"inventory"`
	Equipment     map[string]string `json:"equipment"`
	Unlocks       []string          `json:"unlocks"`
	Quests        []QuestState      `json:"quests"`
}

type QuestObjective struct {
	ObjectiveID string `json:"objective_id"`
	Progress    int    `json:"progress"`
	Required    int    `json:"required"`
}

type QuestState struct {
	QuestID    string           `json:"quest_id"`
	Revision   int64            `json:"revision"`
	Objectives []QuestObjective `json:"objectives"`
	Completed  bool             `json:"completed"`
}

type QuestCommit struct {
	PlayerID         string         `json:"player_id"`
	RoomID           string         `json:"room_id"`
	FencingToken     int64          `json:"fencing_token"`
	ExpectedRevision int64          `json:"expected_revision"`
	OperationID      string         `json:"operation_id"`
	QuestID          string         `json:"quest_id"`
	ObjectiveID      string         `json:"objective_id"`
	Required         int            `json:"required"`
	Amount           int            `json:"amount"`
	Reward           map[string]int `json:"-"`
}

type QuestSnapshot struct {
	PlayerID string   `json:"player_id"`
	Revision int64    `json:"revision"`
	Unlocks  []string `json:"unlocks"`
}
type SessionLease struct {
	PlayerID       string    `json:"player_id"`
	RoomID         string    `json:"room_id"`
	FencingToken   int64     `json:"fencing_token"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}
type ProgressQuery struct {
	PlayerID    string `json:"player_id"`
	OperationID string `json:"operation_id"`
}
type ProgressCommit struct {
	PlayerID         string `json:"player_id"`
	RoomID           string `json:"room_id"`
	FencingToken     int64  `json:"fencing_token"`
	ExpectedRevision int64  `json:"expected_revision"`
	OperationID      string `json:"operation_id"`
	Payload          []byte `json:"payload"`
}
type EquipmentCommit struct {
	PlayerID         string `json:"player_id"`
	RoomID           string `json:"room_id"`
	FencingToken     int64  `json:"fencing_token"`
	ExpectedRevision int64  `json:"expected_revision"`
	OperationID      string `json:"operation_id"`
	Slot             string `json:"slot"`
	ItemID           string `json:"item_id"`
	Equipped         bool   `json:"equipped"`
}
type OperationResult struct {
	OperationID string         `json:"operation_id"`
	Status      string         `json:"status"`
	Code        string         `json:"code,omitempty"`
	Result      string         `json:"result,omitempty"`
	Revision    int64          `json:"revision"`
	Payload     []byte         `json:"payload,omitempty"`
	Inventory   map[string]int `json:"inventory,omitempty"`
}

const OperationSucceeded = "succeeded"
