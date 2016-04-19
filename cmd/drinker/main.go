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
	"strings"
	"time"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/github"
	"github.com/thecodearchive/gitarchive/metrics"
	"github.com/thecodearchive/gitarchive/queue"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	queuePath := flag.String("queue", "./queue.db", "clone queue path or DSN")
	cachePath := flag.String("cache", "./cache.json", "startracker cache path")
	influxAddr := flag.String("influx", "http://localhost:8086", "InfluxDB address")
	camli.AddFlags()
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("usage: drinker 2016-01-02-15")
	}

	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("Please set the env var GITHUB_TOKEN")
	}
	st := github.NewStarTracker(1000000000, os.Getenv("GITHUB_TOKEN"))

	if f, err := os.Open(*cachePath); err != nil {
		log.Println("[ ] Can't load StarTracker cache, starting empty")
	} else {
		log.Println("[+] Loaded StarTracker cache")
		st.LoadCache(f)
		f.Close()
	}

	qDriver := "sqlite3"
	if strings.Index(*queuePath, "@") != -1 {
		qDriver = "mysql"
	}
	log.Printf("[ ] Opening queue (%s)...", qDriver)
	q, err := queue.Open(qDriver, *queuePath)
	fatalIfErr(err)

	defer func() {
		log.Println("[ ] Closing queue...")
		fatalIfErr(q.Close())

		f, err := os.Create(*cachePath)
		fatalIfErr(err)
		log.Println("[ ] Writing StarTracker cache...")
		st.SaveCache(f)
		fatalIfErr(f.Close())
	}()

	exp := expvar.NewMap("drinker")
	expEvents := new(expvar.Map).Init()
	expLatest := new(expvar.String)
	exp.Set("latestevent", expLatest)
	exp.Set("events", expEvents)
	exp.Set("github", st.Expvar())

	err = metrics.StartInfluxExport(*influxAddr, "drinker", exp)
	fatalIfErr(err)

	d := &Drinker{
		q: q, st: st, u: camli.NewUploader(),
		exp: exp, expEvents: expEvents, expLatest: expLatest,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("[ ] Stopping gracefully...")
		d.Stop()
	}()

	t, err := time.Parse(github.HourFormat, flag.Arg(0))
	fatalIfErr(err)

	startTime := t.Add(time.Hour).Add(2 * time.Minute)
	for {
		if time.Now().Before(startTime) {
			log.Printf("[ ] Waiting for the %s archive until %s...",
				t.Format(github.HourFormat), startTime)
			if !interruptableSleep(startTime.Sub(time.Now())) {
				break
			}
		}
		log.Printf("[ ] Opening %s archive download...", t.Format(github.HourFormat))
		a, err := github.DownloadArchive(t)
		fatalIfErr(err) // TODO: make more graceful
		if a == nil {
			exp.Add("archives404", 1)
			startTime = time.Now().Add(2 * time.Minute)
			continue
		}

		log.Printf("[+] Archive %s found, consuming...", t.Format(github.HourFormat))
		err = d.DrinkArchive(a)
		a.Close()
		if err == StoppedError {
			break
		}
		fatalIfErr(err) // TODO: make more graceful

		exp.Add("archivesfinished", 1)
		t = t.Add(time.Hour)
		startTime = t.Add(time.Hour).Add(2 * time.Minute)
	}

	log.Println("[+] Processed events until", expLatest)
	fmt.Print(exp.String())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Panic(err) // panic to let the defer run
	}
}
