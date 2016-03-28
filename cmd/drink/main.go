package main

import (
	"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/github"
	"github.com/thecodearchive/gitarchive/queue"
)

var (
	exp       = expvar.NewMap("drink")
	expEvents = new(expvar.Map).Init()
	expLatest = new(expvar.String)
)

func init() {
	exp.Set("latestevent", expLatest)
	exp.Set("events", expEvents)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	queuePath := flag.String("queue", "./queue.db", "clone queue path")
	cachePath := flag.String("cache", "./cache.json", "startracker cache path")
	camli.AddFlags()
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("usage: drink json.gz")
	}

	uploader := camli.NewUploader()

	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("Please set the env var GITHUB_TOKEN")
	}
	st := github.NewStarTracker(1000000000, os.Getenv("GITHUB_TOKEN"))
	exp.Set("github", st.Expvar())

	if f, err := os.Open(*cachePath); err != nil {
		log.Println("[ ] Can't load StarTracker cache, starting empty")
	} else {
		log.Println("[+] Loaded StarTracker cache")
		st.LoadCache(f)
		f.Close()
	}

	var closing uint32
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("[ ] Terminating gracefully...")
		atomic.StoreUint32(&closing, 1)
	}()

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

	log.Println("[ ] Opening archive...")
	f, err := os.Open(flag.Arg(0))
	fatalIfErr(err)
	defer f.Close()
	r, err := github.NewTimelineArchiveReader(f)
	fatalIfErr(err)
	defer r.Close()

	log.Println("[ ] Reading events...")
	for atomic.LoadUint32(&closing) == 0 {
		var e github.Event
		if err := r.Read(&e); err == io.EOF {
			break
		} else {
			fatalIfErr(err)
		}

		expEvents.Add(e.Type, 1)
		expLatest.Set(e.CreatedAt.String())

		switch e.Type {
		case "PushEvent":
			url := "https://github.com/" + e.Repo.Name + ".git"
			repo, err := uploader.GetRepo(url)
			if err != nil {
				exp.Add("dropped", 1)
				log.Printf("[-] Camli error: %s; dropped event: %#v", err, e)
				continue
			}
			if repo != nil {
				exp.Add("gotrepo", 1)
				exp.Add("queued", 1)
				q.Add(e.Repo.Name, repo.Parent)
			}

			stars, parent, err := st.Get(e.Repo.Name)
			if rate := github.IsRateLimit(err); rate != nil {
				exp.Add("ratehits", 1)
				log.Println("[-] Hit GitHub ratelimits, sleeping until", rate.Reset)
				interruptableSleep(rate.Reset.Sub(time.Now()))

				// TODO: retry, both here and for the other "dropped" cases
				exp.Add("dropped", 1)
				log.Printf("[-] Resuming; dropped event: %#v", e)
				continue
			}
			if github.Is404(err) {
				exp.Add("vanished", 1)
				continue
			}
			if err != nil {
				exp.Add("dropped", 1)
				log.Printf("[-] ST error: %s; dropped event: %#v", err, e)
				continue
			}
			if stars < 10 {
				exp.Add("skipped", 1)
				continue
			}

			exp.Add("queued", 1)
			q.Add(e.Repo.Name, parent)

		case "CreateEvent":
			var ce github.CreateEvent
			fatalIfErr(json.Unmarshal(e.Payload, &ce))
			switch ce.RefType {
			case "repository":
				st.CreateEvent(e.Repo.Name, "", e.CreatedAt.Time)
			case "branch", "tag":
				// TODO
			}

		case "WatchEvent":
			st.WatchEvent(e.Repo.Name, e.CreatedAt.Time)

		case "ForkEvent":
			var fe github.ForkEvent
			fatalIfErr(json.Unmarshal(e.Payload, &fe))
			st.CreateEvent(fe.Forkee.FullName, e.Repo.Name, e.CreatedAt.Time)

		case "DeleteEvent":
			// TODO

		case "PublicEvent":
			// TODO
		}
	}

	log.Println("[+] Processed events until", expLatest)

	fmt.Print(exp.String())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func interruptableSleep(d time.Duration) bool {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer signal.Stop(c)
	t := time.NewTimer(d)
	select {
	case <-c:
		return false
	case <-t.C:
		return true
	}
}
