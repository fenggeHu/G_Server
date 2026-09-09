package domain

import "time"

const (
	Preset  = "exploration"
	Content = "g0-empty-v1"
	Room    = "g0-room"
	Proto   = 1
)

type Player struct {
	ID           string
	PasswordHash string
}
type Session struct{ PlayerID, TokenHash string }
type Ticket struct {
	Token, PlayerID, RoomID, PresetID, ConfigHash string
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
}

type Reservation struct{ RoomID, PlayerID string }
