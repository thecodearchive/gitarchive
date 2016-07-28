package main

import (
	"database/sql"
	"expvar"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	_ "github.com/go-sql-driver/mysql"

	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"

	"github.com/thecodearchive/gitarchive/index"
	"github.com/thecodearchive/gitarchive/metrics"
	"github.com/thecodearchive/gitarchive/queue"
	"github.com/thecodearchive/gitarchive/weekmap"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	schedule, err := weekmap.Parse(MustGetenv("SCHEDULE"))
	fatalIfErr(err)

	exp := expvar.NewMap("fetcher")

	err = metrics.StartInfluxExport(MustGetenv("INFLUX_ADDR"), "fetcher", exp)
	fatalIfErr(err)

	client, err := storage.NewClient(context.Background())
	fatalIfErr(err)

	bucket := client.Bucket(OptGetenv("FETCHER_BUCKET_NAME", "packfiles"))

	log.Println("[ ] Opening db connection...")
	db, err := sql.Open("mysql", MustGetenv("DB_ADDR")+"?parseTime=true")
	fatalIfErr(err)
	defer func() {
		log.Println("[ ] Closing db connection...")
		fatalIfErr(db.Close())
	}()

	log.Println("[ ] Opening queue...")
	q, err := queue.Open(db)
	fatalIfErr(err)
	exp.Set("queuelen", metrics.IntFunc(func() int {
		res, _ := q.Len()
		return res
	}))

	log.Println("[ ] Opening index...")
	i, err := index.Open(db)
	fatalIfErr(err)

	f := &Fetcher{exp: exp, q: q, i: i, bucket: bucket, schedule: schedule}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("[ ] Stopping gracefully...")
		f.Stop()
	}()

	fatalIfErr(f.Run())

	fmt.Print(exp.String())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Panic(err) // panic to let the defer run
	}
}

func MustGetenv(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Panicln("Missing environment variable:", name)
	}
	return val
}

func MustGetenvInt(name string) int {
	i, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		log.Panicln("Missing environment variable:", name)
	}
	return i
}

func OptGetenv(name, defaultVal string) string {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal
	}
	return val
}
