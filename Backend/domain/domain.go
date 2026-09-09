package domain

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
