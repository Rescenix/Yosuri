// internal/database/prismdb.go
package database

import (
	_ "backend/internal/prismd" // 导入驱动，触发 init() 注册
	"database/sql"
	"log"
)

var PrismDB *sql.DB

func InitPrismDB() {
	var err error
	PrismDB, err = sql.Open("prismd", "localhost:5666")
	if err != nil {
		log.Fatal("PrismD 连接失败: ", err)
	}

	PrismDB.SetMaxOpenConns(10)
	PrismDB.SetMaxIdleConns(5)
}
