package main

import (
	"flux/internal/database"
	"flux/webserver"
	"time"
)

func main() {
	time.Sleep(10 * time.Second)
	dsn := "tcp://clickhouse:9000"
	database.InitDatabase(dsn)
	webserver.StartServer()
}
