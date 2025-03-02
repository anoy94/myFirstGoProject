package models

// GlobalConfig 用于读取全局配置文件
type GlobalConfig struct {
	HttpPorts     []string `yaml:"http_ports"`
	RaftPorts     []string `yaml:"raft_ports"`
	SyncMode      string   `yaml:"sync"`
	NewestVersion string   `yaml:"newest_version"`
	MysqlDSN      string   `yaml:"mysql_dsn"`
}
