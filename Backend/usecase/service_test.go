package usecase

import (
	"aigame/server/backend/domain"
	"context"
	"testing"
)

type fakeStore struct {
	player  domain.Player
	session domain.Session
	ticket  domain.Ticket
}

func (f *fakeStore) FindPlayer(context.Context, string) (domain.Player, error) { return f.player, nil }
func (f *fakeStore) CreateSession(context.Context, string, string) error       { return nil }
func (f *fakeStore) FindSession(context.Context, string) (domain.Session, error) {
	return f.session, nil
}
func (f *fakeStore) RevokeSession(context.Context, string) error { return nil }
func (f *fakeStore) IssueTicket(_ context.Context, t domain.Ticket, _, _ string) error {
	f.ticket = t
	return nil
}
func (f *fakeStore) RedeemTicket(context.Context, domain.Ticket) (domain.Ticket, error) {
	return f.ticket, nil
}
func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) RegisterRoom(context.Context, domain.RoomRecord) (domain.RoomRecord, error) {
	return domain.RoomRecord{}, nil
}
func (f *fakeStore) HeartbeatRoom(context.Context, string, int, int, int, string) error { return nil }
func (f *fakeStore) AllocateRoom(context.Context, string, string, int, string) (domain.RoomRecord, error) {
	return domain.RoomRecord{}, nil
}
func (f *fakeStore) ReleaseReservation(context.Context, string, string) error { return nil }
func (f *fakeStore) AcquireSession(context.Context, string, string) (domain.SessionLease, error) {
	return domain.SessionLease{}, nil
}
func (f *fakeStore) RenewSession(context.Context, domain.SessionLease) (domain.SessionLease, error) {
	return domain.SessionLease{}, nil
}
func (f *fakeStore) ReleaseSession(context.Context, domain.SessionLease) error { return nil }
func (f *fakeStore) CommitProgress(context.Context, domain.ProgressCommit) (domain.OperationResult, error) {
	return domain.OperationResult{}, nil
}
func (f *fakeStore) QueryProgress(context.Context, domain.ProgressQuery) (domain.OperationResult, error) {
	return domain.OperationResult{}, nil
}
func (f *fakeStore) GetProgressSnapshot(context.Context, string) (domain.ProgressSnapshot, error) {
	return domain.ProgressSnapshot{}, nil
}
func (f *fakeStore) CommitEquipment(context.Context, domain.EquipmentCommit) (domain.ProgressSnapshot, error) { return domain.ProgressSnapshot{}, nil }
func TestConfigHashStableAndTicketBindsPlayer(t *testing.T) {
	f := &fakeStore{}
	s := &Service{Store: f}
	if ConfigHash() != ConfigHash() {
		t.Fatal("unstable hash")
	}
	x, e := s.Ticket(context.Background(), "player", "session-hash", domain.Preset, domain.Content, domain.Proto)
	if e != nil || x.PlayerID != "player" || f.ticket.PlayerID != "player" {
		t.Fatalf("bad ticket: %+v %v", x, e)
	}
}

func TestCoopCombatPresetIsSupportedAndBoundToTicket(t *testing.T) {
	f := &fakeStore{}
	s := &Service{Store: f}
	x, err := s.Ticket(context.Background(), "player", "session-hash", "coop_combat", "g4-coop-combat-v1", domain.Proto)
	if err != nil {
		t.Fatalf("coop_combat rejected: %v", err)
	}
	if x.PresetID != "coop_combat" || x.Content != "g4-coop-combat-v1" || x.ConfigHash == "" {
		t.Fatalf("ticket is not bound to combat preset: %+v", x)
	}
}
