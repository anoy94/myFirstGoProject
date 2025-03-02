package models

type RaftCommand struct {
	Operation string      `json:"operation"`
	Key       string      `json:"key,omitempty"`
	Value     interface{} `json:"value,omitempty"`
	Lesson    string      `json:"lesson,omitempty"`
	Score     string      `json:"score,omitempty"`
}
