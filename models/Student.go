package models

type Student struct {
	ID     string            `json:"id"` //给变量添加标记键
	Name   string            `json:"name"`
	Sex    string            `json:"sex"`
	Class  string            `json:"class"`
	Scores map[string]string `json:"scores"`
}
