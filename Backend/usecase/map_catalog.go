package usecase

import (
	"aigame/server/backend/domain"
	"sort"
)

// MapDefinition is the server-controlled identity of a playable map.
// Visual resources are intentionally outside this type.
type MapDefinition struct {
	ID               string
	ContentVersion   string
	AuthorityVersion string
	APIVersion       string
	MinEngineVersion string
	// PackageURL/SHA256/Signature describe the distributable world pack.
	// Empty means the map ships with the base client and has no separate pack.
	PackageURL string
	SHA256     string
	Signature  string
}

// MapPackage is the client-facing release manifest entry.
type MapPackage struct {
	ID               string `json:"id"`
	ContentVersion   string `json:"content_version"`
	AuthorityVersion string `json:"authority_version"`
	APIVersion       string `json:"api_version"`
	MinEngineVersion string `json:"min_engine_version"`
	URL              string `json:"url"`
	SHA256           string `json:"sha256"`
	Signature        string `json:"signature"`
}

var mapCatalog = map[string]MapDefinition{
	domain.DefaultMap: {
		ID:               domain.DefaultMap,
		ContentVersion:   domain.DefaultMapContent,
		AuthorityVersion: domain.DefaultMapAuthority,
		APIVersion:       domain.DefaultAPIVersion,
		MinEngineVersion: domain.DefaultMinEngineVersion,
	},
}

// MapIDs returns registered map ids in stable order.
func MapIDs() []string {
	ids := make([]string, 0, len(mapCatalog))
	for id := range mapCatalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// MapPackages returns the release manifest for all registered maps.
func MapPackages() []MapPackage {
	ids := MapIDs()
	out := make([]MapPackage, 0, len(ids))
	for _, id := range ids {
		value := mapCatalog[id]
		out = append(out, MapPackage{
			ID:               value.ID,
			ContentVersion:   value.ContentVersion,
			AuthorityVersion: value.AuthorityVersion,
			APIVersion:       value.APIVersion,
			MinEngineVersion: value.MinEngineVersion,
			URL:              value.PackageURL,
			SHA256:           value.SHA256,
			Signature:        value.Signature,
		})
	}
	return out
}

func ValidMap(id, contentVersion, authorityVersion string) bool {
	value, ok := mapCatalog[id]
	return ok && value.ContentVersion == contentVersion && value.AuthorityVersion == authorityVersion
}

func MapDefinitionFor(id string) (MapDefinition, bool) {
	value, ok := mapCatalog[id]
	return value, ok
}

func DefaultMapDefinition() MapDefinition { return mapCatalog[domain.DefaultMap] }
