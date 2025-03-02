package models

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"github.com/goccy/go-json"
	"github.com/hashicorp/raft"
	"golang.org/x/net/context"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	ctx  = context.Background()
	Lock sync.RWMutex
)

type DB struct {
	DBConn     *gorm.DB
	Students   map[string]*Student
	VersionMap map[string]string
	Config     *NodeConfig
	Raft       *raft.Raft
}

func NewDB(cfg *NodeConfig, dbConn *gorm.DB) *DB {
	return &DB{
		DBConn:     dbConn,
		Students:   make(map[string]*Student),
		VersionMap: make(map[string]string),
		Config:     cfg,
	}
}

func (echoDB *DB) VersionMapAddOrUpdate(key string, value string) {
	Lock.Lock()
	defer Lock.Unlock()
	echoDB.VersionMap[key] = value
}

func (echoDB *DB) VersionMapGet(key string) string {
	return echoDB.VersionMap[key]
}

func (echoDB *DB) VersionMapDel(key string) {
	Lock.Lock()
	defer Lock.Unlock()
	delete(echoDB.VersionMap, key)
}

func (echoDB *DB) VersionMapNum() int {
	return len(echoDB.VersionMap)
}

func (echoDB *DB) AddOrUpdateStudent(student *Student) {
	if echoDB.Config.SyncMode == "raft" {
		//cmd := RaftCommand{
		//	Operation: "AddOrUpdateStudent",
		//	Value:     *student,
		//}
		//data, _ := json.Marshal(cmd)
		//if echoDB.Raft.State() == raft.Leader {
		//	echoDB.Raft.Apply(data, 10*time.Second)
		//} else {
		//	// 转发到 leader：由 Raft 通信端口映射到 HTTP 端口
		//	leaderAddr := string(echoDB.Raft.Leader())
		//	if leaderAddr == "" {
		//		log.Printf("未获取到 leader 地址")
		//		return
		//	}
		//	parts := strings.Split(leaderAddr, ":")
		//	if len(parts) != 2 {
		//		log.Printf("leader 地址格式错误: %s", leaderAddr)
		//		return
		//	}
		//	raftLeaderPort, err := strconv.Atoi(parts[1])
		//	if err != nil {
		//		log.Printf("解析 leader Raft 端口失败: %v", err)
		//		return
		//	}
		//	httpLeaderPort := raftLeaderPort + 4080
		//	url := fmt.Sprintf("http://127.0.0.1:%d/raft/apply", httpLeaderPort)
		//	http.Post(url, "application/json", bytes.NewReader(data))
		//}
	} else if echoDB.Config.SyncMode == "gossip" {
		Lock.Lock()
		echoDB.Students[student.ID] = student
		Lock.Unlock()
	} else {
		return
	}
}

func (echoDB *DB) GetStudent(id string) *Student {
	//检查学生是否存在
	Lock.RLock()
	student, exists := echoDB.Students[id]
	Lock.RUnlock()
	if !exists {
		return nil
	}
	return student
}

func (echoDB *DB) DeleteStudent(id string) {
	if echoDB.Config.SyncMode == "raft" {
		//cmd := RaftCommand{
		//	Operation: "DeleteStudent",
		//	Key:       id,
		//}
		//data, _ := json.Marshal(cmd)
		//if echoDB.Raft.State() == raft.Leader {
		//	echoDB.Raft.Apply(data, 10*time.Second)
		//} else {
		//	// 转发到 leader：由 Raft 通信端口映射到 HTTP 端口
		//	leaderAddr := string(echoDB.Raft.Leader())
		//	if leaderAddr == "" {
		//		log.Printf("未获取到 leader 地址")
		//		return
		//	}
		//	parts := strings.Split(leaderAddr, ":")
		//	if len(parts) != 2 {
		//		log.Printf("leader 地址格式错误: %s", leaderAddr)
		//		return
		//	}
		//	raftLeaderPort, err := strconv.Atoi(parts[1])
		//	if err != nil {
		//		log.Printf("解析 leader Raft 端口失败: %v", err)
		//		return
		//	}
		//	httpLeaderPort := raftLeaderPort + 4080
		//	url := fmt.Sprintf("http://127.0.0.1:%d/raft/apply", httpLeaderPort)
		//	http.Post(url, "application/json", bytes.NewReader(data))
		//}
	} else if echoDB.Config.SyncMode == "gossip" {
		Lock.Lock()
		delete(echoDB.Students, id)
		Lock.Unlock()
	} else {
		return
	}
}

func (echoDB *DB) UploadInfoFromCSV(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	//延迟关闭文件
	defer file.Close()
	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	//创建传输学生信息的通道
	infoCh := make(chan *Student, len(rows))
	for _, row := range rows {
		wg.Add(1)
		go func(row []string) { //存入学生信息的匿名函数
			defer wg.Done()
			//排除缺少必要信息的行
			if len(row) < 4 {
				fmt.Printf("信息缺失: %v\n", row)
				return
			}
			student := &Student{
				ID:     row[0],
				Name:   row[1],
				Sex:    row[2],
				Class:  row[3],
				Scores: make(map[string]string),
			}
			if len(row) > 4 {
				for i := 4; i < len(row); i += 2 {
					// 跳过有缺失的课程和分数，防止指针溢出导致程序崩溃
					if i+1 >= len(row) {
						fmt.Printf("课程或分数信息不完整: %v\n", row)
						break
					}
					lesson := row[i]
					score := row[i+1]
					student.Scores[lesson] = score
				}
			}
			// 将学生信息发送到通道
			infoCh <- student
		}(row)
	}
	//关闭通道
	go func() {
		wg.Wait()
		close(infoCh)
	}()
	//写入学生信息
	for student := range infoCh {
		echoDB.AddOrUpdateStudent(student)
	}
	return nil
}

func (echoDB *DB) AddOrUpdateScore(id string, lesson string, score string) {
	if echoDB.Config.SyncMode == "raft" {
		//cmd := RaftCommand{
		//	Operation: "AddOrUpdateScore",
		//	Key:       id,
		//	Lesson:    lesson,
		//	Score:     score,
		//}
		//data, _ := json.Marshal(cmd)
		//if echoDB.Raft.State() == raft.Leader {
		//	echoDB.Raft.Apply(data, 10*time.Second)
		//} else {
		//	// 转发到 leader：由 Raft 通信端口映射到 HTTP 端口
		//	leaderAddr := string(echoDB.Raft.Leader())
		//	if leaderAddr == "" {
		//		log.Printf("未获取到 leader 地址")
		//		return
		//	}
		//	parts := strings.Split(leaderAddr, ":")
		//	if len(parts) != 2 {
		//		log.Printf("leader 地址格式错误: %s", leaderAddr)
		//		return
		//	}
		//	raftLeaderPort, err := strconv.Atoi(parts[1])
		//	if err != nil {
		//		log.Printf("解析 leader Raft 端口失败: %v", err)
		//		return
		//	}
		//	httpLeaderPort := raftLeaderPort + 4080
		//	url := fmt.Sprintf("http://127.0.0.1:%d/raft/apply", httpLeaderPort)
		//	http.Post(url, "application/json", bytes.NewReader(data))
		//}
	} else if echoDB.Config.SyncMode == "gossip" {
		Lock.Lock()
		echoDB.Students[id] = &Student{
			ID:     id,
			Scores: make(map[string]string),
		}
		echoDB.Students[id].Scores[lesson] = score
		Lock.Unlock()
	}

}

func (echoDB *DB) GetScore(id string, lesson string) string {
	Lock.RLock()
	defer Lock.RUnlock()
	return echoDB.Students[id].Scores[lesson]

}

func (echoDB *DB) DeleteScore(id string, lesson string) {
	if echoDB.Config.SyncMode == "raft" {
		//cmd := RaftCommand{
		//	Operation: "DeleteScore",
		//	Key:       id,
		//	Lesson:    lesson,
		//}
		//data, _ := json.Marshal(cmd)
		//if echoDB.Raft.State() == raft.Leader {
		//	echoDB.Raft.Apply(data, 10*time.Second)
		//} else {
		//	// 转发到 leader：由 Raft 通信端口映射到 HTTP 端口
		//	leaderAddr := string(echoDB.Raft.Leader())
		//	if leaderAddr == "" {
		//		log.Printf("未获取到 leader 地址")
		//		return
		//	}
		//	parts := strings.Split(leaderAddr, ":")
		//	if len(parts) != 2 {
		//		log.Printf("leader 地址格式错误: %s", leaderAddr)
		//		return
		//	}
		//	raftLeaderPort, err := strconv.Atoi(parts[1])
		//	if err != nil {
		//		log.Printf("解析 leader Raft 端口失败: %v", err)
		//		return
		//	}
		//	httpLeaderPort := raftLeaderPort + 4080
		//	url := fmt.Sprintf("http://127.0.0.1:%d/raft/apply", httpLeaderPort)
		//	http.Post(url, "application/json", bytes.NewReader(data))
		//}
	} else if echoDB.Config.SyncMode == "gossip" {
		Lock.Lock()
		if student, exists := echoDB.Students[id]; exists {
			delete(student.Scores, lesson)
		}
		Lock.Unlock()
	}
}

// gossip 定期向其他节点传播版本信息

func (echoDB *DB) StartGossip() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	// 处理本地事件同步
	for range ticker.C {
		echoDB.SyncToPeers()
	}
}

// 数据同步
func (echoDB *DB) SyncToPeers() {
	peers := echoDB.Config.Peers
	randIndex := rand.Intn(len(peers))
	p := peers[randIndex]
	url := fmt.Sprintf("http://localhost:%s/sync", p)
	syncData := SyncData{
		Students:   echoDB.Students,
		VersionMap: echoDB.VersionMap,
	}
	data, _ := json.Marshal(syncData)

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("Sync to %s failed: %v", p, err)
		return
	}
	defer resp.Body.Close()
}

// 将DB结构体中的各个字段序列化为JSON
func (db *DB) ToSqlDB() (*SqlDB, error) {
	// 序列化Students map为JSON
	studentsJSON, err := json.Marshal(db.Students)
	if err != nil {
		log.Printf("序列化Students失败: %v", err)
		return nil, err
	}

	// 序列化VersionMap map为JSON
	versionMapJSON, err := json.Marshal(db.VersionMap)
	if err != nil {
		log.Printf("序列化VersionMap失败: %v", err)
		return nil, err
	}

	// 序列化Config结构体为JSON
	configJSON, err := json.Marshal(db.Config)
	if err != nil {
		log.Printf("序列化Config失败: %v", err)
		return nil, err
	}

	return &SqlDB{
		Students:   string(studentsJSON),
		VersionMap: string(versionMapJSON),
		Config:     string(configJSON),
	}, nil
}

func InitMySQL(cfg *NodeConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.MysqlDSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// 反序列化为DB
func (sqlDB *SqlDB) ToDB() (*DB, error) {
	// 反序列化Students
	students := make(map[string]*Student)
	if err := json.Unmarshal([]byte(sqlDB.Students), &students); err != nil {
		return nil, fmt.Errorf("反序列化Students失败: %v", err)
	}

	// 反序列化VersionMap
	versionMap := make(map[string]string)
	if err := json.Unmarshal([]byte(sqlDB.VersionMap), &versionMap); err != nil {
		return nil, fmt.Errorf("反序列化VersionMap失败: %v", err)
	}

	// 反序列化Config
	var config NodeConfig
	if err := json.Unmarshal([]byte(sqlDB.Config), &config); err != nil {
		return nil, fmt.Errorf("反序列化Config失败: %v", err)
	}

	return &DB{
		Students:   students,
		VersionMap: versionMap,
		Config:     &config,
	}, nil
}

// 从SqlDB中读取JSON字符串并反序列化为DB结构体的对应字段
func JsonToStudent(JsonStudnet *StudentToJSON) (*Student, error) {
	// 反序列化scores
	Scores := make(map[string]string)
	if err := json.Unmarshal([]byte(JsonStudnet.Scores), &Scores); err != nil {
		return nil, fmt.Errorf("反序列化scores失败: %v", err)
	}

	return &Student{
		ID:     JsonStudnet.ID,
		Name:   JsonStudnet.Name,
		Sex:    JsonStudnet.Sex,
		Class:  JsonStudnet.Class,
		Scores: Scores,
	}, nil
}

// 数据库缓存预热
func (echoDB *DB) loadStudentFromMySQL(id string) (*StudentToJSON, error) {
	// 从 MySQL 中加载数据
	var JsonStudent StudentToJSON
	if err := echoDB.DBConn.First(&JsonStudent, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &JsonStudent, nil
}

func (echoDB *DB) LoadDataFromCache(id string) (*Student, error) {
	// 从 echoDB 中加载数据（内存缓存）
	Lock.RLock()
	student, exists := echoDB.Students[id]
	Lock.RUnlock()
	if !exists {
		// 如果数据没有，进行懒加载
		JsonStudent, err := echoDB.loadStudentFromMySQL(id)
		if err != nil {
			return nil, err
		}
		student, err := JsonToStudent(JsonStudent)
		if err != nil {
			return nil, err
		}
		// 加载到 echoDB 中
		echoDB.AddOrUpdateStudent(student)
		return student, nil
	}
	return student, nil
}

func (echoDB *DB) PreloadHotData() {
	//从 MySQL 中加载一些热点数据到 echoDB
	var studentsJSON []StudentToJSON
	if err := echoDB.DBConn.Limit(100).Find(&studentsJSON).Error; err != nil {
		log.Println("预加载数据失败:", err)
		return
	}

	// 将热点数据缓存到 echoDB
	for _, JsonStudnet := range studentsJSON {
		student, err := JsonToStudent(&JsonStudnet)
		if err != nil {
			return
		}
		// 加载到 echoDB 中
		echoDB.AddOrUpdateStudent(student)
	}
}

//func WaitForLeader(r *raft.Raft, timeout time.Duration) (raft.ServerAddress, error) {
//	deadline := time.Now().Add(timeout)
//	for time.Now().Before(deadline) {
//		if leader := r.Leader(); leader != "" {
//			// 增加Leader可用性检查
//			conn, err := net.DialTimeout("tcp", string(leader), 500*time.Millisecond)
//			if err == nil {
//				conn.Close()
//				return leader, nil
//			}
//		}
//		time.Sleep(100 * time.Millisecond)
//	}
//	return "", fmt.Errorf("leader not elected within %v", timeout)
//}
//
//// Raft初始化完整实现
//func (echoDB *DB) InitRaft(cfg *NodeConfig) error {
//	// 创建自定义logger
//	logger := hclog.New(&hclog.LoggerOptions{
//		Name:   "raft",
//		Output: os.Stderr,
//		Level:  hclog.Info,
//	})
//
//	raftConfig := raft.DefaultConfig()
//	raftConfig.LocalID = raft.ServerID(cfg.Raft.ID)
//	raftConfig.Logger = logger
//
//	// 创建TCP传输层
//	addr := fmt.Sprintf("localhost:%s", cfg.Raft.Port)
//	transport, err := raft.NewTCPTransport(
//		addr,
//		nil,
//		10,
//		10*time.Second,
//		io.Discard,
//	)
//	if err != nil {
//		return fmt.Errorf("创建传输层失败: %v", err)
//	}
//
//	dataDir := filepath.Join("raft", cfg.Raft.ID)
//	os.MkdirAll(dataDir, 0755) // 确保目录存在
//
//	// 更新快照和日志存储路径
//	snapshots, err := raft.NewFileSnapshotStoreWithLogger(
//		filepath.Join(dataDir, "snapshots"),
//		2,
//		logger,
//	)
//
//	// 创建日志存储
//	logStore, err := raftboltdb.NewBoltStore(
//		filepath.Join(dataDir, "raft.db"),
//	)
//
//	if err != nil {
//		return fmt.Errorf("创建日志存储失败: %v", err)
//	}
//
//	// 创建Raft实例
//	raftInstance, err := raft.NewRaft(
//		raftConfig,
//		echoDB,
//		logStore,
//		logStore,
//		snapshots,
//		transport,
//	)
//	if err != nil {
//		return fmt.Errorf("初始化Raft失败: %v", err)
//	}
//
//	// 构建集群配置
//	var servers []raft.Server
//	for _, raftPort := range cfg.RaftPeers {
//		serverID := fmt.Sprintf("node_%s", raftPort)
//		servers = append(servers, raft.Server{
//			ID:      raft.ServerID(serverID),
//			Address: raft.ServerAddress(fmt.Sprintf("localhost:%s", raftPort)),
//		})
//	}
//
//	// 引导集群
//	if cfg.Raft.ID == "node_"+cfg.RaftPeers[0] {
//		config := raft.Configuration{Servers: servers}
//		err := raftInstance.BootstrapCluster(config).Error()
//		if err != nil && strings.Contains(err.Error(), "bootstrap only works on new clusters") {
//			log.Printf("集群已引导，跳过 bootstrap")
//		} else if err != nil {
//			return fmt.Errorf("Bootstrap error: %v", err)
//		}
//	}
//
//	echoDB.Raft = raftInstance
//	return nil
//}
//
//// 实现raft.FSM接口
//func (echoDB *DB) Apply(l *raft.Log) interface{} {
//	var cmd RaftCommand
//	if err := json.Unmarshal(l.Data, &cmd); err != nil {
//		log.Printf("Failed to unmarshal command: %v", err)
//		return nil
//	}
//
//	lock.Lock()
//	defer lock.Unlock()
//
//	switch cmd.Operation {
//	case "AddOrUpdateStudent":
//		var student Student
//		tmpValue, _ := json.Marshal(cmd.Value)
//		err := json.Unmarshal(tmpValue, &student)
//		if err != nil {
//			return nil
//		}
//		echoDB.Students[student.ID] = &student
//
//	case "DeleteStudent":
//		delete(echoDB.Students, cmd.Key)
//
//	case "AddOrUpdateScore":
//		if student, exists := echoDB.Students[cmd.Key]; exists {
//			student.Scores[cmd.Lesson] = cmd.Score
//		}
//
//	case "DeleteScore":
//		if student, exists := echoDB.Students[cmd.Key]; exists {
//			delete(student.Scores, cmd.Lesson)
//		}
//
//	case "UpdateVersion":
//		echoDB.VersionMap[cmd.Key] = cmd.Value.(string)
//
//	default:
//		log.Printf("Unknown operation: %s", cmd.Operation)
//	}
//	return nil
//}
//
//// 实现FSM Snapshot接口
//func (echoDB *DB) Snapshot() (raft.FSMSnapshot, error) {
//	lock.RLock()
//	defer lock.RUnlock()
//
//	return &fsmSnapshot{
//		Students:   echoDB.Students,
//		VersionMap: echoDB.VersionMap,
//	}, nil
//}
//
//type fsmSnapshot struct {
//	Students   map[string]*Student
//	VersionMap map[string]string
//}
//
//func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
//	data, err := json.Marshal(s)
//	if err != nil {
//		sink.Cancel()
//		return err
//	}
//
//	if _, err := sink.Write(data); err != nil {
//		sink.Cancel()
//		return err
//	}
//
//	return sink.Close()
//}
//
//func (s *fsmSnapshot) Release() {}
//
//// 实现FSM Restore接口
//func (echoDB *DB) Restore(r io.ReadCloser) error {
//	var newState struct {
//		Students   map[string]*Student
//		VersionMap map[string]string
//	}
//
//	if err := json.NewDecoder(r).Decode(&newState); err != nil {
//		return err
//	}
//
//	lock.Lock()
//	defer lock.Unlock()
//
//	echoDB.Students = newState.Students
//	echoDB.VersionMap = newState.VersionMap
//	return nil
//}
//
//// SetupRouter 注册所有 RESTful 路由，使用传入的 echoDB 实例
//func SetupRouter(echoDB *DB) *gin.Engine {
//	r := gin.Default()
//
//	// 学生信息管理路由组
//	info := r.Group("/info")
//	{
//		info.POST("/", func(c *gin.Context) {
//			var student Student
//			if err := c.ShouldBindJSON(&student); err != nil {
//				c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
//				return
//			}
//			echoDB.AddOrUpdateStudent(&student)
//			c.JSON(http.StatusOK, gin.H{"message": "success"})
//		})
//
//		info.GET("/:id", func(c *gin.Context) {
//			id := c.Param("id")
//			student, err := echoDB.LoadDataFromCache(id)
//			if err != nil || student == nil {
//				c.JSON(http.StatusNotFound, gin.H{"error": "该学生不存在"})
//			} else {
//				c.JSON(http.StatusOK, gin.H{"message": student})
//			}
//		})
//
//		info.DELETE("/:id", func(c *gin.Context) {
//			id := c.Param("id")
//			echoDB.DeleteStudent(id)
//			c.JSON(http.StatusOK, gin.H{"message": "success"})
//		})
//
//		info.POST("/uploadcsv", func(c *gin.Context) {
//			file, err := c.FormFile("file")
//			if err != nil {
//				c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
//				return
//			}
//			filePath := fmt.Sprintf("%s/%s", os.TempDir(), file.Filename)
//			if err := c.SaveUploadedFile(file, filePath); err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
//				return
//			}
//			if err := echoDB.UploadInfoFromCSV(filePath); err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"error": "文件加载失败"})
//				return
//			}
//			c.JSON(http.StatusOK, gin.H{"message": "学生信息上传成功"})
//		})
//	}
//
//	// 学生成绩管理路由组
//	score := r.Group("/score")
//	{
//		score.POST("/:id/:lesson/:score", func(c *gin.Context) {
//			id := c.Param("id")
//			lesson := c.Param("lesson")
//			scoreVal := c.Param("score")
//			echoDB.AddOrUpdateScore(id, lesson, scoreVal)
//			c.JSON(http.StatusOK, gin.H{"message": "success"})
//		})
//
//		score.GET("/:id/:lesson", func(c *gin.Context) {
//			id := c.Param("id")
//			lesson := c.Param("lesson")
//			scoreVal := echoDB.GetScore(id, lesson)
//			if scoreVal == "" {
//				c.JSON(http.StatusNotFound, gin.H{"message": "该课程成绩不存在"})
//			} else {
//				c.JSON(http.StatusOK, gin.H{"message": scoreVal})
//			}
//		})
//
//		score.DELETE("/:id/:lesson", func(c *gin.Context) {
//			id := c.Param("id")
//			lesson := c.Param("lesson")
//			echoDB.DeleteScore(id, lesson)
//			c.JSON(http.StatusOK, gin.H{"message": "success"})
//		})
//	}
//
//	// 版本检查接口
//	r.GET("/check-update", func(c *gin.Context) {
//		currentVersion := c.Query("current_version")
//		needsUpdate := currentVersion != echoDB.VersionMap["newestVision"]
//		c.JSON(http.StatusOK, gin.H{
//			"code":    "200",
//			"message": "success",
//			"data": gin.H{
//				"current_version": currentVersion,
//				"newest_version":  echoDB.VersionMap["newestVision"],
//				"needs_update":    needsUpdate,
//			},
//		})
//	})
//
//	// Raft 相关接口
//	r.POST("/raft/apply", func(c *gin.Context) {
//		if echoDB.Config.SyncMode != "raft" {
//			c.JSON(http.StatusBadRequest, gin.H{"error": "当前模式不支持 raft"})
//			return
//		}
//		data, _ := io.ReadAll(c.Request.Body)
//		future := echoDB.Raft.Apply(data, 10*time.Second)
//		if err := future.Error(); err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
//			return
//		}
//		c.JSON(http.StatusOK, gin.H{"result": future.Response()})
//	})
//
//	r.POST("/raft/join", func(c *gin.Context) {
//		var req struct {
//			Port string `json:"port"`
//		}
//		if err := c.ShouldBindJSON(&req); err != nil {
//			c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求"})
//			return
//		}
//		serverID := fmt.Sprintf("node_%s", req.Port)
//		serverAddr := fmt.Sprintf(":%s", req.Port)
//		future := echoDB.Raft.AddVoter(
//			raft.ServerID(serverID),
//			raft.ServerAddress(serverAddr),
//			0,
//			0,
//		)
//		if err := future.Error(); err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
//			return
//		}
//		c.JSON(http.StatusOK, gin.H{"message": "节点加入成功"})
//	})
//
//	return r
//}
