package maxmind

import (
	"github.com/oschwald/maxminddb-golang"
	"log"
	"net"
	"os"
)

var db *maxminddb.Reader

func init() {
	var err error
	maxMindFilePath := os.Getenv("MAXMIND_DB_PATH")
	if maxMindFilePath == "" {
		log.Fatalf("MAXMIND_DB_PATH environment variable not set")
	}
	db, err = maxminddb.Open(maxMindFilePath)
	if err != nil {
		log.Fatal(err)
	}

}

func LookupIp(ip net.IP) (GeoData, error) {
	var err error
	var record GeoData
	err = db.Lookup(ip, &record)
	if err != nil {
		log.Panic(err)
	}
	return record, nil
}
