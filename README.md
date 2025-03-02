在第一次作业代码基础上拓展<br>
实现了多实例部署，检查版本更新等功能，内存数据库名acheDB，存储students，config配置文件和version版本等信息<br>
使用多线程的锁模型和MYSQL数据源，可更改config.yaml文件改变端口，如MYSQL数据库的DSN和数据同步算法等，符合可插拔式配置<br>
通信协议继续用GIN<br>
<br>
<br>
更新部分：<br>
成功实现了数据库缓存和预热的功能，raft算法太难了，写了半天放弃了<br>
代码中大段注释为原来raft代码部分，没删，看以后再学学能不能补上，这次作业就不写了<br>
以下为代码要点<br>
acheDB的功能靠结构体DB实现，故主要的业务实现都在DB.go中<br>
4个version方法实现version的增删查改统计功能，然后是student,scorce的增删查改功能和从CSV批量导入的功能<br>
StartGossip是gossip算法方法<br>
SyncToPeers为具体实现<br>
由于DB有map，gorm无法直接序列化和反序列化，故用JsonToStudent，ToSqlDB和ToDB来回转化为SqlDB，从而能够存入mysql<br>
LoadDataFromCache方法先从内存数据库查找，没有则从mysql数据库查找<br>
PreloadHotData实现启动时的缓存预热，只先加载一部分热点 key<br>


