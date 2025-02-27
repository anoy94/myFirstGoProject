package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"myCode/models"
	"net/http"
	"os"
)

// Student 定义学生信息和成绩的结构体

func main() {

	//读取配置文件
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return
	}
	var cfg models.Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return
	}
	var echoDB = models.NewDB(&cfg)

	// 服务器上的最新版本号
	echoDB.VersionMapAddOrUpdate("newestVision", cfg.NewestVersion)
	go echoDB.StartGossip()

	// 初始化 MySQL
	_, err = models.InitMySQL(&cfg)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 加载热点数据到 echoDB 中
	//go echoDB.PreloadHotData()

	r := gin.Default()

	//该项目采用RESTful规范

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
			var student *models.Student = echoDB.GetStudent(id)

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
	//数据同步的接口，使用gossip
	r.POST("sync", func(c *gin.Context) {
		if err := c.ShouldBindJSON(&echoDB); err != nil {
			//请求不合法
			c.JSON(http.StatusBadRequest, gin.H{"error": "数据同步请求格式错误"})
			return
		}

	})

	// 在peer中的端口运行
	for _, peer := range echoDB.Config.Peers {
		go func(peer string) {
			if err := r.Run(":" + peer); err != nil {
				log.Fatalf("Server failed to start on peer %s: %v", peer, err)
			}
		}(peer)
	}
	if err := r.Run(":" + echoDB.Config.Port); err != nil {
		log.Fatalf("Server failed to start on peer %s: %v", echoDB.Config.Port, err)
	}

}
