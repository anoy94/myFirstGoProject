package models

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"github.com/goccy/go-json"
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
	db   *gorm.DB
	ctx  = context.Background()
	lock sync.RWMutex
)

type DB struct {
	Students   map[string]*Student
	VersionMap map[string]string
	Config     *Config
}

func NewDB(cfg *Config) *DB {
	return &DB{
		Students:   make(map[string]*Student),
		VersionMap: make(map[string]string),
		Config:     cfg,
	}
}

func (echoDB *DB) VersionMapAddOrUpdate(key string, value string) {
	lock.Lock()
	defer lock.Unlock()
	echoDB.VersionMap[key] = value
}

func (echoDB *DB) VersionMapGet(key string) string {
	return echoDB.VersionMap[key]
}

func (echoDB *DB) VersionMapDel(key string) {
	lock.Lock()
	defer lock.Unlock()
	delete(echoDB.VersionMap, key)
}

func (echoDB *DB) VersionMapNum() int {
	return len(echoDB.VersionMap)
}

func (echoDB *DB) AddOrUpdateStudent(student *Student) {
	lock.Lock()
	echoDB.Students[student.ID] = student
	lock.Unlock()
}

func (echoDB *DB) GetStudent(id string) *Student {
	//检查学生是否存在
	lock.RLock()
	student, exists := echoDB.Students[id]
	lock.RUnlock()
	if !exists {
		return nil
	}
	return student
}

func (echoDB *DB) DeleteStudent(id string) {
	lock.Lock()
	delete(echoDB.Students, id)
	lock.Unlock()
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
	lock.Lock()
	echoDB.Students[id].Scores[lesson] = score
	lock.Unlock()
}

func (echoDB *DB) GetScore(id string, lesson string) string {
	lock.RLock()
	defer lock.RUnlock()
	return echoDB.Students[id].Scores[lesson]

}

func (echoDB *DB) DeleteScore(id string, lesson string) {
	lock.RLock()
	delete(echoDB.Students[id].Scores, lesson)
	lock.RUnlock()
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
	var p = echoDB.Config.Peers[rand.Intn(len(echoDB.Config.Peers))]
	url := fmt.Sprintf("http://%s/sync", p)
	data, _ := json.Marshal(echoDB)
	//向其他端口发送请求进行传染
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("Sync to %s failed: %v", p, err)
		return
	}
	defer resp.Body.Close()
}

func InitMySQL(cfg *Config) (*gorm.DB, error) {
	var err error
	db, err = gorm.Open(mysql.Open(cfg.MysqlDSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

//func (echoDB *DB) loadDataFromMySQL(id string) (*Student, error) {
//	// 从 MySQL 中加载数据
//	var student Student
//	if err := db.First(&student, "id = ?", id).Error; err != nil {
//		return nil, err
//	}
//	return &student, nil
//}
//
//func (echoDB *DB) loadDataFromCache(id string) (*Student, error) {
//	// 从 echoDB 中加载数据（内存缓存）
//	lock.RLock()
//	student, exists := echoDB.Students[id]
//	lock.RUnlock()
//	if !exists {
//		// 如果数据没有，进行懒加载
//		student, err := echoDB.loadDataFromMySQL(id)
//		if err != nil {
//			return nil, err
//		}
//		// 加载到 echoDB 中
//		echoDB.AddOrUpdateStudent(student)
//	}
//	return student, nil
//}
//
//func (echoDB *DB) PreloadHotData() {
//	// 假设从 MySQL 中加载一些热点数据到 echoDB
//	var students []Student
//	if err := db.Limit(100).Find(&students).Error; err != nil {
//		log.Println("预加载数据失败:", err)
//		return
//	}
//
//	// 将热点数据缓存到 echoDB
//	for _, student := range students {
//		echoDB.AddOrUpdateStudent(&student)
//	}
//}
