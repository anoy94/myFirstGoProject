
pic.zip为运行截图，学生信息管理.apifox.json为用apifox打开的接口文档，csvfortest.txt为测试csv上传功能的示例文件，code.zip为源代码<br>
代码采用RESFul规范<br>
因为代码体量小所以全写在main.go里，没有分开<br>
实现了学生信息和成绩的录入、删除、修改和查询功能以及从 CSV 文件中批量导入学生信息到系统的功能<br>
并在现有系统中添加异常处理机制，确保系统在遇到错误时能够返回适当的错误信息。<br>
CSV文件上传功能有一些小问题，需要每行学科数量相同才能上传成功，不然uploadInfoFromCSV方法会返回错误，改不好
