package main

import (
	"expvar"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
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

	var t time.Time
	resume, err := ioutil.ReadFile(MustGetenv("RESUME_PATH"))
	if os.IsNotExist(err) {
		t = time.Now().Truncate(time.Hour).Add(-12 * time.Hour)
		log.Println("[ ] Can't load resume file, starting 12 hours ago")
	} else {
		fatalIfErr(err)
		t, err = time.Parse(github.HourFormat, string(resume))
		fatalIfErr(err)
		log.Printf("[+] Resuming from %s", t.Format(github.HourFormat))
	}

	exp := expvar.NewMap("drinker")
	expEvents := new(expvar.Map).Init()
	expLatest := new(expvar.String)
	exp.Set("latestevent", expLatest)
	exp.Set("events", expEvents)

	err = metrics.StartInfluxExport(MustGetenv("INFLUX_ADDR"), "drinker", exp)
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

	st := github.NewStarTracker(10000000, MustGetenv("GITHUB_TOKEN"))
	exp.Set("github", st.Expvar())
	if f, err := os.Open(MustGetenv("CACHE_PATH")); err != nil {
		log.Println("[ ] Can't load StarTracker cache, starting empty")
	} else {
		fatalIfErr(st.LoadCache(f))
		f.Close()
		log.Println("[+] Loaded StarTracker cache")
	}
	defer func() {
		f, err := os.Create(MustGetenv("CACHE_PATH"))
		fatalIfErr(err)
		log.Println("[ ] Writing StarTracker cache...")
		st.SaveCache(f)
		fatalIfErr(f.Close())
	}()

	d := &Drinker{
		q: q, st: st, u: u,
		exp: exp, expEvents: expEvents, expLatest: expLatest,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("[ ] Stopping gracefully...")
		d.Stop()
	}()

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
		fatalIfErr(ioutil.WriteFile(MustGetenv("RESUME_PATH"),
			[]byte(t.Format(github.HourFormat)), 0664))
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

func MustGetenv(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Panicln("Missing environment variable:", name)
	}
	return val
}
