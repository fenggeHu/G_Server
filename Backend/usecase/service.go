package usecase

import (
	"aigame/server/backend/domain"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/alexedwards/argon2id"
)

var ErrUnauthorized = errors.New("unauthorized")

type Store interface {
	FindPlayer(context.Context, string) (domain.Player, error)
	CreateSession(context.Context, string, string) error
	FindSession(context.Context, string) (domain.Session, error)
	RevokeSession(context.Context, string) error
	IssueTicket(context.Context, domain.Ticket, string, string) error
	RedeemTicket(context.Context, domain.Ticket) (domain.Ticket, error)
	RegisterRoom(context.Context, domain.RoomRecord) (domain.RoomRecord, error)
	HeartbeatRoom(context.Context, string, int, int, int, string) error
	AllocateRoom(context.Context, string, string, int, string) (domain.RoomRecord, error)
	ReleaseReservation(context.Context, string, string) error
	Ping(context.Context) error
}
type Service struct {
	Store        Store
	ServiceToken string
	Host         string
	Port         int
}

func Hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func ConfigHash() string {
	b, _ := json.Marshal(struct {
		Content string `json:"gameplay_content_hash"`
		Max     int    `json:"max_players"`
		Preset  string `json:"preset_id"`
		Proto   int    `json:"protocol_version"`
	}{domain.Content, 8, domain.Preset, domain.Proto})
	return Hash(string(b))
}
func token(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *Service) Login(ctx context.Context, u, p string) (string, string, error) {
	x, e := s.Store.FindPlayer(ctx, u)
	if e != nil {
		return "", "", e
	}
	ok, _ := argon2id.ComparePasswordAndHash(p, x.PasswordHash)
	if !ok {
		return "", "", ErrUnauthorized
	}
	t := token(32)
	if e = s.Store.CreateSession(ctx, x.ID, Hash(t)); e != nil {
		return "", "", e
	}
	return t, x.ID, nil
}
func (s *Service) Auth(ctx context.Context, t string) (domain.Session, error) {
	return s.Store.FindSession(ctx, Hash(t))
}
func (s *Service) Ticket(ctx context.Context, playerID, sessionHash, preset, content string, proto int) (domain.Ticket, error) {
	if preset != domain.Preset || content != domain.Content || proto != domain.Proto {
		return domain.Ticket{}, errors.New("version mismatch")
	}
	t := token(48)
	x := domain.Ticket{Token: t, PlayerID: playerID, RoomID: domain.Room, PresetID: domain.Preset, ConfigHash: ConfigHash(), Protocol: domain.Proto, Content: domain.Content}
	return x, s.Store.IssueTicket(ctx, x, Hash(t), sessionHash)
}
func (s *Service) Allocate(ctx context.Context, playerID, sessionHash, preset, content string, proto int) (domain.Ticket, domain.RoomRecord, error) {
	if preset != domain.Preset || content != domain.Content || proto != domain.Proto {
		return domain.Ticket{}, domain.RoomRecord{}, errors.New("version mismatch")
	}
	r, err := s.Store.AllocateRoom(ctx, playerID, content, proto, ConfigHash())
	if err != nil {
		return domain.Ticket{}, domain.RoomRecord{}, err
	}
	t := token(48)
	x := domain.Ticket{Token: t, PlayerID: playerID, RoomID: r.ID, PresetID: preset, ConfigHash: ConfigHash(), Protocol: proto, Content: content}
	return x, r, s.Store.IssueTicket(ctx, x, Hash(t), sessionHash)
}
func (s *Service) RegisterRoom(ctx context.Context, r domain.RoomRecord) (domain.RoomRecord, error) {
	if r.Capacity < 1 || r.Capacity > 8 {
		return r, errors.New("invalid capacity")
	}
	return s.Store.RegisterRoom(ctx, r)
}
func (s *Service) Redeem(ctx context.Context, x domain.Ticket) (domain.Ticket, error) {
	if x.RoomID != domain.Room || x.Protocol != domain.Proto || x.Content != domain.Content || x.ConfigHash != ConfigHash() {
		return domain.Ticket{}, ErrUnauthorized
	}
	return s.Store.RedeemTicket(ctx, x)
}
