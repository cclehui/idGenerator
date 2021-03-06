package persistent

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"strconv"
)

var db *sql.DB

func GetMysqlDB(userName string, password string,
	host string, port int, dbName string, maxIdleCon int, maxOpenCon int) *sql.DB {

	//单例
	if db != nil {
		return db
	}

	var connectStr string

	connectStr = userName + ":" + password +
		"@tcp(" + host + ":" + strconv.Itoa(port) +
		")/" + dbName + "?charset=utf8"

	var err error

	db, err = sql.Open("mysql", connectStr)

	if err != nil {
		panic(err.Error())
	}

	//sql 连接池功能
	db.SetMaxIdleConns(maxIdleCon) //最大空闲连接数
	db.SetMaxOpenConns(maxOpenCon) //最大能打开的连接数

	//defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err.Error())
	}

	return db
}
