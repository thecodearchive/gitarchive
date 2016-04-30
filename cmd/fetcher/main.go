package main

import (
	"expvar"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/thecodearchive/gitarchive/camli"
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

	u, err := camli.NewUploader()
	fatalIfErr(err)

	qDriver := "sqlite3"
	if strings.Index(MustGetenv("QUEUE_ADDR"), "@") != -1 {
		qDriver = "mysql"
	}
	log.Printf("[ ] Opening queue (%s)...", qDriver)
	q, err := queue.Open(qDriver, MustGetenv("QUEUE_ADDR"))
	fatalIfErr(err)
	defer func() {
		log.Println("[ ] Closing queue...")
		fatalIfErr(q.Close())
	}()

	f := &Fetcher{exp: exp, q: q, u: u, schedule: schedule}

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
