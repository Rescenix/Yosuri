// backend/internal/database/mysql.go
package database

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func InitDB() {
	var err error
	dsn := "root:Gdx9pyrz.@tcp(127.0.0.1:3306)/shanca_blog?parseTime=true&charset=utf8mb4"
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("数据库连接失败: ", err)
	}

	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(5)
}
