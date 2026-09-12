package usecase

import (
	"aigame/server/backend/domain"
	"testing"
)

func TestMapPackagesExposeReleaseManifest(t *testing.T) {
	packages := MapPackages()
	if len(packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(packages))
	}
	first := packages[0]
	if first.ID != domain.DefaultMap || first.ContentVersion != domain.DefaultMapContent || first.AuthorityVersion != domain.DefaultMapAuthority {
		t.Fatalf("unexpected identity: %+v", first)
	}
	if first.APIVersion == "" || first.MinEngineVersion == "" {
		t.Fatalf("missing version metadata: %+v", first)
	}
}

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
