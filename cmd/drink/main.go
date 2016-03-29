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
	"time"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/github"
	"github.com/thecodearchive/gitarchive/queue"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	queuePath := flag.String("queue", "./queue.db", "clone queue path")
	cachePath := flag.String("cache", "./cache.json", "startracker cache path")
	camli.AddFlags()
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("usage: drink 2016-01-02-15")
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

	log.Println("[ ] Opening queue...")
	q, err := queue.Open(*queuePath)
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

	exp := expvar.NewMap("drink")
	expEvents := new(expvar.Map).Init()
	expLatest := new(expvar.String)
	exp.Set("latestevent", expLatest)
	exp.Set("events", expEvents)
	exp.Set("github", st.Expvar())

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

	log.Println("[ ] Opening archive download...")
	a, err := github.DownloadArchive(t)
	fatalIfErr(err)
	defer a.Close()
	log.Println("[ ] Consuming...")
	fatalIfErr(d.DrinkArchive(a))

	log.Println("[+] Processed events until", expLatest)
	fmt.Print(exp.String())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Panic(err) // panic to let the defer run
	}
}
