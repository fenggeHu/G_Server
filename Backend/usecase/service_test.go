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
