package main

import (
	"database/sql"
	"expvar"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/thecodearchive/gitarchive/index"
	"github.com/thecodearchive/gitarchive/metrics"
	"github.com/thecodearchive/go-trello"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	exp := expvar.NewMap("backpanel")

	err := metrics.StartInfluxExport(MustGetenv("INFLUX_ADDR"), "backpanel", exp)
	fatalIfErr(err)

	token := MustGetenv("TRELLO_TOKEN")
	cl, err := trello.NewAuthClient(MustGetenv("TRELLO_KEY"), &token)
	fatalIfErr(err)

	pause, err := time.ParseDuration(OptGetenv("INTERVAL", "60s"))
	fatalIfErr(err)

	blacklistID := MustGetenv("BLACKLIST_BOARD")

	log.Println("[ ] Opening db connection...")
	db, err := sql.Open("mysql", MustGetenv("DB_ADDR")+"?parseTime=true")
	fatalIfErr(err)
	defer func() {
		log.Println("[ ] Closing db connection...")
		fatalIfErr(db.Close())
	}()

	log.Println("[ ] Opening index...")
	i, err := index.Open(db)
	fatalIfErr(err)

	b := &Backpanel{exp: exp, c: cl, i: i, pause: pause, blacklistID: blacklistID}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("[ ] Stopping gracefully...")
		b.Stop()
	}()

	log.Println("[ ] Running backpanel loop...")
	fatalIfErr(b.Run())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Printf("%+v", err)
		panic("fatal error") // panic to let the defer run
	}
}

func MustGetenv(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Panicln("Missing environment variable:", name)
	}
	return val
}

func OptGetenv(name, defaultVal string) string {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal
	}
	return val
}
