package models

// NodeConfig 为每个实例生成独立配置
type NodeConfig struct {
	Port          string   // 当前节点 HTTP 端口
	Peers         []string // 所有 HTTP 节点端口
	SyncMode      string
	NewestVersion string
	MysqlDSN      string     `yaml:"mysql_dsn"`
	Raft          RaftConfig // 当前节点的 Raft 配置
	RaftPeers     []string   // 所有 Raft 节点端口
}
