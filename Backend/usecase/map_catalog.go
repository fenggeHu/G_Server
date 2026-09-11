package usecase

import "aigame/server/backend/domain"

// MapDefinition is the server-controlled identity of a playable map.
// Visual resources are intentionally outside this type.
type MapDefinition struct {
	ID               string
	ContentVersion   string
	AuthorityVersion string
}

var mapCatalog = map[string]MapDefinition{
	domain.DefaultMap: {ID: domain.DefaultMap, ContentVersion: domain.DefaultMapContent, AuthorityVersion: domain.DefaultMapAuthority},
}

func MapIDs() []string { return []string{domain.DefaultMap} }

func ValidMap(id, contentVersion, authorityVersion string) bool {
	value, ok := mapCatalog[id]
	return ok && value.ContentVersion == contentVersion && value.AuthorityVersion == authorityVersion
}

func MapDefinitionFor(id string) (MapDefinition, bool) {
	value, ok := mapCatalog[id]
	return value, ok
}
