package models

type StudentToJSON struct {
	ID     string `gorm:"primaryKey;column:id;type:varchar(40)"`
	Name   string `gorm:"column:name;type:varchar(100)"`
	Sex    string `gorm:"column:sex;type:varchar(100)"`
	Class  string `gorm:"column:class;type:varchar(50)"`
	Scores string `gorm:"column:scores;type:json"`
}

// 指定表名
func (StudentToJSON) TableName() string {
	return "students"
}
