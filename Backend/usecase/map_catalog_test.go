package usecase

import (
	"aigame/server/backend/domain"
	"testing"
)

func TestMapPackagesExposeReleaseManifest(t *testing.T) {
	packages := MapPackages()
	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}
	byID := map[string]MapPackage{}
	for _, p := range packages {
		byID[p.ID] = p
	}
	base, ok := byID[domain.DefaultMap]
	if !ok || base.ContentVersion != domain.DefaultMapContent || base.AuthorityVersion != domain.DefaultMapAuthority {
		t.Fatalf("unexpected default identity: %+v", base)
	}
	if base.APIVersion == "" || base.MinEngineVersion == "" {
		t.Fatalf("missing version metadata: %+v", base)
	}
	forest, ok := byID[domain.ForestMap]
	if !ok || forest.ContentVersion != domain.ForestMapContent || forest.AuthorityVersion != domain.ForestMapAuthority {
		t.Fatalf("unexpected forest identity: %+v", forest)
	}
	if forest.APIVersion == "" || forest.MinEngineVersion == "" {
		t.Fatalf("missing forest version metadata: %+v", forest)
	}
}

func TestMapCatalogValidatesVersionedIdentity(t *testing.T) {
	if !ValidMap(domain.DefaultMap, domain.DefaultMapContent, domain.DefaultMapAuthority) {
		t.Fatal("default map should be valid")
	}
	if !ValidMap(domain.ForestMap, domain.ForestMapContent, domain.ForestMapAuthority) {
		t.Fatal("forest map should be valid")
	}
	if ValidMap(domain.ForestMap, domain.DefaultMapContent, domain.ForestMapAuthority) {
		t.Fatal("forest with default content version should be rejected")
	}
	if ValidMap(domain.DefaultMap, "old", domain.DefaultMapAuthority) {
		t.Fatal("stale content version should be rejected")
	}
	if ValidMap("missing", domain.DefaultMapContent, domain.DefaultMapAuthority) {
		t.Fatal("unknown map should be rejected")
	}
}
