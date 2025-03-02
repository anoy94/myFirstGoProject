package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"io/ioutil"
	"log"
	"myCode/models"
	"net/http"
	"os"
)

var mysqlDB *gorm.DB

func main() {

	//读取配置文件
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}
	var g models.GlobalConfig
	err = yaml.Unmarshal(data, &g)
	if err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	cfg := &models.NodeConfig{
		Port:          g.HttpPorts[0],
		Peers:         g.HttpPorts,
		SyncMode:      g.SyncMode,
		NewestVersion: g.NewestVersion,
		MysqlDSN:      g.MysqlDSN,
		Raft: models.RaftConfig{
			ID:   "node_" + g.RaftPorts[0],
			Port: g.RaftPorts[0],
		},
		RaftPeers: g.RaftPorts,
	}
	// 初始化 MySQL
	mysqlDB, err := models.InitMySQL(cfg)
	if err != nil {
		fmt.Printf("数据库连接失败: ", err)
	}
	echoDB := models.NewDB(cfg, mysqlDB)

	// 服务器上的最新版本号
	echoDB.VersionMapAddOrUpdate("newestVision", cfg.NewestVersion)

	//数据同步
	if len(cfg.Peers) == 0 && cfg.SyncMode == "gossip" {
		log.Fatal("gossip模式需要配置至少一个peer节点")
	}
	if cfg.SyncMode == "raft" {
		//if err := echoDB.InitRaft(cfg); err != nil {
		//	log.Fatalf("节点 %s Raft初始化失败: %v", cfg.Port, err)
		//}
		//// 等待 Leader 选举
		//leader, err := models.WaitForLeader(echoDB.Raft, 10*time.Second)
		//if err != nil {
		//	log.Printf("Leader 选举超时: %v", err)
		//}
	} else if cfg.SyncMode == "gossip" {
		go echoDB.StartGossip()
	}

	//加载热点数据到 echoDB 中
	go echoDB.PreloadHotData()

	r := gin.Default()

	//管理学生基本信息的路由组
	g1 := r.Group("/info")
	{
		//增加或修改学生信息的路由
		g1.POST("/", func(c *gin.Context) {
			var student models.Student
			if err := c.ShouldBindJSON(&student); err != nil {
				//请求不合法
				c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
				return
			}
			echoDB.AddOrUpdateStudent(&student)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		//查询学生信息的路由
		g1.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			student, _ := echoDB.LoadDataFromCache(id)

			if student == nil {
				//学生不存在，返回报错
				c.JSON(http.StatusNotFound, gin.H{"error": "该学生不存在"})
			} else {
				c.JSON(http.StatusOK, gin.H{"message": student})
			}

		})

		//删除学生信息的路由
		g1.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			echoDB.DeleteStudent(id)
			c.JSON(http.StatusOK, gin.H{
				"message": "success",
			})
		})

		//从csv文件批量导入学生信息的路由
		g1.POST("/uploadcsv", func(c *gin.Context) {
			file, err := c.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
				return
			}
			//动态创建路径保存csv文件
			filePath := fmt.Sprintf("%s/%s", os.TempDir(), file.Filename)
			if err := c.SaveUploadedFile(file, filePath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
				return
			}
			if err := echoDB.UploadInfoFromCSV(filePath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "文件后端加载失败"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "学生信息上传成功"})
		})
	}

	//管理学生成绩的路由组
	g2 := r.Group("/score")
	{
		//增加或修改学生成绩的路由
		g2.POST("/:id/:lesson/:score", func(c *gin.Context) {
			id := c.Param("id")
			lesson := c.Param("lesson")
			score := c.Param("score")
			// 检查学生是否存在
			if echoDB.Students[id] == nil {
				// 如果学生不存在，初始化新学生
				echoDB.Students[id] = &models.Student{
					ID:     id,
					Scores: make(map[string]string),
				}
			}
			// 如果Scores未初始化，则初始化
			if echoDB.Students[id].Scores == nil {
				echoDB.Students[id].Scores = make(map[string]string)
			}
			echoDB.AddOrUpdateScore(id, lesson, score)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		//查询学生成绩的路由
		g2.GET("/:id/:lesson", func(c *gin.Context) {
			id := c.Param("id")
			lesson := c.Param("lesson")
			var score string = echoDB.GetScore(id, lesson)
			if score == "" {
				c.JSON(http.StatusNotFound, gin.H{"message": "该课程成绩不存在"}) //检查lesson是否存在
			} else {
				c.JSON(http.StatusOK, gin.H{"message": score})
			}

		})

		//删除学生成绩的路由
		g2.DELETE("/:id/:lesson", func(c *gin.Context) {
			id := c.Param("id")
			lesson := c.Param("lesson")
			echoDB.DeleteScore(id, lesson)
			c.JSON(http.StatusOK, gin.H{
				"message": "success",
			})
		})
	}
	//检查版本的接口
	r.GET("/check-update", func(context *gin.Context) {
		// 获取客户端传递的当前版本号
		currentVersion := context.Query("current_version")
		// 比较版本号
		needsUpdate := currentVersion != echoDB.VersionMap["newestVision"]
		// 返回结果
		context.JSON(200, gin.H{
			"code":    "200",
			"message": "success",
			"data": gin.H{
				"current_version": currentVersion,
				"newest_version":  echoDB.VersionMap["newestVision"],
				"needs_update":    needsUpdate,
			},
		})
	})
	////数据同步的接口，使用gossip
	//r.POST("sync", func(c *gin.Context) {
	//	if err := c.ShouldBindJSON(&echoDB); err != nil {
	//		//请求不合法
	//		c.JSON(http.StatusBadRequest, gin.H{"error": "数据同步请求格式错误"})
	//		return
	//	}
	//
	//})
	r.POST("/sync", func(c *gin.Context) {
		var syncData models.SyncData
		if err := c.ShouldBindJSON(&syncData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "数据同步请求格式错误"})
			return
		}
		// 合并同步数据到当前节点
		models.Lock.Lock()
		// 合并学生数据
		for id, student := range syncData.Students {
			if _, exists := echoDB.Students[id]; !exists {
				echoDB.Students[id] = student
			} else {
				// 合并或更新逻辑（根据需求调整）
				echoDB.Students[id].Name = student.Name
				echoDB.Students[id].Sex = student.Sex
				echoDB.Students[id].Class = student.Class
				for lesson, score := range student.Scores {
					echoDB.Students[id].Scores[lesson] = score
				}
			}
		}
		// 合并版本信息
		for key, value := range syncData.VersionMap {
			echoDB.VersionMap[key] = value
		}
		models.Lock.Unlock()

		c.JSON(http.StatusOK, gin.H{"message": "同步成功"})
	})
	//r.POST("/raft/apply", func(c *gin.Context) {
	//	if echoDB.Config.SyncMode != "raft" {
	//		c.JSON(http.StatusBadRequest, gin.H{"error": "当前模式不支持raft"})
	//		return
	//	}
	//
	//	data, _ := io.ReadAll(c.Request.Body)
	//	future := echoDB.Raft.Apply(data, 10*time.Second)
	//	if err := future.Error(); err != nil {
	//		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	//		return
	//	}
	//	c.JSON(http.StatusOK, gin.H{"result": future.Response()})
	//})
	//r.POST("/raft/join", func(c *gin.Context) {
	//	var req struct {
	//		Port string `json:"port"`
	//	}
	//	if err := c.ShouldBindJSON(&req); err != nil {
	//		c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求"})
	//		return
	//	}
	//
	//	serverID := fmt.Sprintf("node_%s", req.Port)
	//	serverAddr := fmt.Sprintf("localhost:%s", req.Port)
	//
	//	future := echoDB.Raft.AddVoter(
	//		raft.ServerID(serverID),
	//		raft.ServerAddress(serverAddr),
	//		0,
	//		0,
	//	)
	//
	//	if err := future.Error(); err != nil {
	//		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	//		return
	//	}
	//
	//	c.JSON(http.StatusOK, gin.H{"message": "节点加入成功"})
	//})

	// 在peer中的端口运行
	for _, peer := range echoDB.Config.Peers {
		go func(peer string) {
			if err := r.Run(":" + peer); err != nil {
				log.Fatalf("Server failed to start on peer %s: %v", peer, err)
			}
		}(peer)
	}
	if err := r.Run(":8000"); err != nil {
		log.Fatalf("Server failed to start on peer 8000: %v", err)
	}

	//// 检查端口数量是否匹配
	//if len(globalCfg.HttpPorts) != len(globalCfg.RaftPorts) {
	//	log.Fatalf("HTTP端口数量(%d)与Raft端口数量(%d)不匹配",
	//		len(globalCfg.HttpPorts), len(globalCfg.RaftPorts))
	//}
	//
	//// 使用 WaitGroup 等待所有节点启动
	//var wg sync.WaitGroup
	//
	//// 遍历所有节点（下标 i 对应一对 HTTP 和 Raft 端口）
	//for i := range globalCfg.HttpPorts {
	//	go func(i int) {
	//		defer wg.Done()
	//		// 构造每个节点的配置
	//		nodeCfg := &models.NodeConfig{
	//			Port:          globalCfg.HttpPorts[i],
	//			Peers:         globalCfg.HttpPorts,
	//			SyncMode:      globalCfg.SyncMode,
	//			NewestVersion: globalCfg.NewestVersion,
	//			MysqlDSN:      globalCfg.MysqlDSN,
	//			Raft: models.RaftConfig{
	//				ID:   "node_" + globalCfg.RaftPorts[i],
	//				Port: globalCfg.RaftPorts[i],
	//			},
	//			RaftPeers: globalCfg.RaftPorts,
	//		}
	//
	//		// 初始化 MySQL（注意：实际部署中可考虑复用连接池）
	//		var mysqlDB *gorm.DB
	//		mysqlDB, err = models.InitMySQL(nodeCfg)
	//		if err != nil {
	//			log.Fatalf("节点 %s 数据库连接失败: %v", nodeCfg.Port, err)
	//		}
	//
	//		// 创建 DB 实例
	//		echoDB := models.NewDB(nodeCfg, mysqlDB)
	//		echoDB.VersionMapAddOrUpdate("newestVision", nodeCfg.NewestVersion)
	//
	//		// 数据同步：根据 sync 模式初始化 raft 或 gossip
	//		if nodeCfg.SyncMode == "raft" {
	//			if err := echoDB.InitRaft(nodeCfg); err != nil {
	//				log.Fatalf("节点 %s Raft初始化失败: %v", nodeCfg.Port, err)
	//			}
	//		} else {
	//			go echoDB.StartGossip()
	//		}
	//
	//		// 执行数据库迁移、预加载数据等
	//		if err := mysqlDB.AutoMigrate(&models.StudentToJSON{}); err != nil {
	//			log.Printf("节点 %s 数据库迁移失败: %v", nodeCfg.Port, err)
	//		}
	//		go echoDB.PreloadHotData()
	//
	//		// 启动 HTTP 服务（每个节点独立注册路由）
	//		router := models.SetupRouter(echoDB)
	//		log.Printf("节点启动：HTTP %s ；Raft %s", nodeCfg.Port, nodeCfg.Raft.Port)
	//		if err := router.Run(":" + nodeCfg.Port); err != nil {
	//			log.Fatalf("节点 %s 启动失败: %v", nodeCfg.Port, err)
	//		}
	//
	//	}(i)
	//}
	//select {}
}
