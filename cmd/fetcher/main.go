package main

import (
	"expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/metrics"
	"github.com/thecodearchive/gitarchive/queue"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	queuePath := flag.String("queue", "./queue.db", "clone queue path")
	influxAddr := flag.String("influx", "http://localhost:8086", "InfluxDB address")
	camli.AddFlags()
	flag.Parse()

	log.Println("[ ] Opening queue...")
	q, err := queue.Open(*queuePath)
	fatalIfErr(err)

	defer func() {
		log.Println("[ ] Closing queue...")
		fatalIfErr(q.Close())
	}()

	exp := expvar.NewMap("fetcher")

	err = metrics.StartInfluxExport(*influxAddr, "fetcher", exp)
	fatalIfErr(err)

	f := &Fetcher{exp: exp, q: q, u: camli.NewUploader()}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
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
