package models

type SqlDB struct {
	ID         uint   `gorm:"primaryKey"`
	Students   string `gorm:"type:text"`
	VersionMap string `gorm:"type:text"`
	Config     string `gorm:"type:text"`
}
