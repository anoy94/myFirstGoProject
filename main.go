package main

import (
	"encoding/csv"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"sync"
)

// Student 定义学生信息和成绩的结构体
type Student struct {
	ID     string            `json:"id"` //给变量添加标记键
	Name   string            `json:"name"`
	Sex    string            `json:"sex"`
	Class  string            `json:"class"`
	Scores map[string]string `json:"scores"`
}

// 创建用于存储数据的map
var students = make(map[string]*Student)
var lock sync.RWMutex

//以下为实现项目所需要的函数， main函数从第129行开始

func addOrUpdateStudent(student *Student) {
	lock.Lock()
	students[student.ID] = student
	lock.Unlock()
}

func getStudent(id string) *Student {
	//检查学生是否存在
	lock.RLock()
	student, exists := students[id]
	lock.RUnlock()
	if !exists {
		return nil
	}
	return student
}

func deleteStudent(id string) {
	lock.Lock()
	delete(students, id)
	lock.Unlock()
}

func uploadInfoFromCSV(filePath string) error {
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
		addOrUpdateStudent(student)
	}
	return nil
}

func addOrUpdateScore(id string, lesson string, score string) {
	lock.RLock()
	students[id].Scores[lesson] = score
	lock.RUnlock()
}

func getScore(id string, lesson string) string {
	lock.RLock()
	defer lock.RUnlock()
	return students[id].Scores[lesson]

}

func deleteScore(id string, lesson string) {
	lock.RLock()
	delete(students[id].Scores, lesson)
	lock.RUnlock()
}

func main() {
	r := gin.Default()

	//该项目采用RESTful规范

	//管理学生基本信息的路由组
	g1 := r.Group("/info")
	{
		//增加或修改学生信息的路由
		g1.POST("/", func(c *gin.Context) {
			var student Student
			if err := c.ShouldBindJSON(&student); err != nil {
				//请求不合法
				c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
				return
			}
			addOrUpdateStudent(&student)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		//查询学生信息的路由
		g1.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var student *Student = getStudent(id)

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
			deleteStudent(id)
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
			if err := uploadInfoFromCSV(filePath); err != nil {
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
			if students[id] == nil {
				// 如果学生不存在，初始化新学生
				students[id] = &Student{
					ID:     id,
					Scores: make(map[string]string),
				}
			}
			// 如果Scores未初始化，则初始化
			if students[id].Scores == nil {
				students[id].Scores = make(map[string]string)
			}
			addOrUpdateScore(id, lesson, score)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		//查询学生成绩的路由
		g2.GET("/:id/:lesson", func(c *gin.Context) {
			id := c.Param("id")
			lesson := c.Param("lesson")
			var score string = getScore(id, lesson)
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
			deleteScore(id, lesson)
			c.JSON(http.StatusOK, gin.H{
				"message": "success",
			})
		})
	}

	//输出全部学生数据到终端的路由，用来测试其他路由
	r.POST("/printfall", func(c *gin.Context) {
		for i := range students {
			fmt.Printf("%v\n", i)
		}
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	err := r.Run(":8000")
	if err != nil {
		return
	}
}
