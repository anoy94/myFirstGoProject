package models

type SyncData struct {
	Students   map[string]*Student `json:"students"`
	VersionMap map[string]string   `json:"versionMap"`
}
