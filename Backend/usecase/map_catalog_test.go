package usecase

import (
	"aigame/server/backend/domain"
	"testing"
)

func TestMapCatalogValidatesVersionedIdentity(t *testing.T) {
	if !ValidMap(domain.DefaultMap, domain.DefaultMapContent, domain.DefaultMapAuthority) {
		t.Fatal("default map should be valid")
	}
	if ValidMap(domain.DefaultMap, "old", domain.DefaultMapAuthority) {
		t.Fatal("stale content version should be rejected")
	}
	if ValidMap("missing", domain.DefaultMapContent, domain.DefaultMapAuthority) {
		t.Fatal("unknown map should be rejected")
	}
}
