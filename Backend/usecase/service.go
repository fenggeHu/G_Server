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
	ProgressStore
}
type Service struct {
	Store        Store
	ServiceToken string
	Host         string
	Port         int
}

func Hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func ConfigHash() string {
	return ConfigHashFor(domain.Preset, domain.Content)
}
func ConfigHashFor(preset, content string) string {
	b, _ := json.Marshal(struct {
		Content string `json:"gameplay_content_hash"`
		Max     int    `json:"max_players"`
		Preset  string `json:"preset_id"`
		Proto   int    `json:"protocol_version"`
	}{content, 8, preset, domain.Proto})
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
	if !validPreset(preset, content, proto) {
		return domain.Ticket{}, errors.New("version mismatch")
	}
	t := token(48)
	x := domain.Ticket{Token: t, PlayerID: playerID, RoomID: domain.Room, PresetID: preset, ConfigHash: ConfigHashFor(preset, content), Protocol: proto, Content: content}
	return x, s.Store.IssueTicket(ctx, x, Hash(t), sessionHash)
}
func validPreset(preset, content string, proto int) bool {
	return proto == domain.Proto && ((preset == domain.Preset && content == domain.Content) || (preset == domain.CombatPreset && content == domain.CombatContent))
}
func ValidPreset(preset, content string, proto int) bool { return validPreset(preset, content, proto) }
func (s *Service) Allocate(ctx context.Context, playerID, sessionHash, preset, content string, proto int) (domain.Ticket, domain.RoomRecord, error) {
	if !validPreset(preset, content, proto) {
		return domain.Ticket{}, domain.RoomRecord{}, errors.New("version mismatch")
	}
	r, err := s.Store.AllocateRoom(ctx, playerID, content, proto, ConfigHashFor(preset, content))
	if err != nil {
		return domain.Ticket{}, domain.RoomRecord{}, err
	}
	t := token(48)
	x := domain.Ticket{Token: t, PlayerID: playerID, RoomID: r.ID, PresetID: preset, ConfigHash: ConfigHashFor(preset, content), Protocol: proto, Content: content}
	err = s.Store.IssueTicket(ctx, x, Hash(t), sessionHash)
	if err != nil {
		_ = s.Store.ReleaseReservation(ctx, r.ID, playerID)
	}
	return x, r, err
}
func (s *Service) RegisterRoom(ctx context.Context, r domain.RoomRecord) (domain.RoomRecord, error) {
	preset := domain.Preset
	if r.GameplayContentHash == domain.CombatContent {
		preset = domain.CombatPreset
	}
	if r.Capacity < 1 || r.Capacity > 8 || r.ProtocolVersion != domain.Proto || (r.GameplayContentHash != domain.Content && r.GameplayContentHash != domain.CombatContent) || r.ResolvedConfigHash != ConfigHashFor(preset, r.GameplayContentHash) || r.Status != "ready" {
		return r, errors.New("invalid capacity")
	}
	return s.Store.RegisterRoom(ctx, r)
}
func (s *Service) Redeem(ctx context.Context, x domain.Ticket) (domain.Ticket, error) {
	if x.RoomID == "" || !validPreset(x.PresetID, x.Content, x.Protocol) || x.ConfigHash != ConfigHashFor(x.PresetID, x.Content) {
		return domain.Ticket{}, ErrUnauthorized
	}
	return s.Store.RedeemTicket(ctx, x)
}

type ProgressStore interface {
	AcquireSession(context.Context, string, string) (domain.SessionLease, error)
	RenewSession(context.Context, domain.SessionLease) (domain.SessionLease, error)
	ReleaseSession(context.Context, domain.SessionLease) error
	CommitProgress(context.Context, domain.ProgressCommit) (domain.OperationResult, error)
	QueryProgress(context.Context, domain.ProgressQuery) (domain.OperationResult, error)
	GetProgressSnapshot(context.Context, string) (domain.ProgressSnapshot, error)
	CommitEquipment(context.Context, domain.EquipmentCommit) (domain.ProgressSnapshot, error)
}

func (s *Service) RenewSession(ctx context.Context, x domain.SessionLease) (domain.SessionLease, error) {
	return s.Store.RenewSession(ctx, x)
}
func (s *Service) ReleaseSession(ctx context.Context, x domain.SessionLease) error {
	return s.Store.ReleaseSession(ctx, x)
}
func (s *Service) QueryProgress(ctx context.Context, x domain.ProgressQuery) (domain.OperationResult, error) {
	return s.Store.QueryProgress(ctx, x)
}

func (s *Service) GetProgressSnapshot(ctx context.Context, playerID string) (domain.ProgressSnapshot, error) {
	return s.Store.GetProgressSnapshot(ctx, playerID)
}

func (s *Service) CommitEquipment(ctx context.Context, x domain.EquipmentCommit) (domain.ProgressSnapshot, error) {
	return s.Store.CommitEquipment(ctx, x)
}

func (s *Service) AcquireSession(ctx context.Context, playerID, roomID string) (domain.SessionLease, error) {
	return s.Store.AcquireSession(ctx, playerID, roomID)
}
func (s *Service) CommitProgress(ctx context.Context, c domain.ProgressCommit) (domain.OperationResult, error) {
	return s.Store.CommitProgress(ctx, c)
}
