package models

type Config struct {
	Port          string   `yaml:"port"`
	Sync          string   `yaml:"sync"`
	Peers         []string `yaml:"peers"`
	NewestVersion string   `yaml:"newest_version"`
	MysqlDSN      string   `yaml:"mysql_dsn"`
}
