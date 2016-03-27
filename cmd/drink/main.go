package main

import (
	"encoding/json"
	"expvar"
	"flag"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/github"
	"github.com/thecodearchive/gitarchive/queue"
)

var (
	exp       = expvar.NewMap("drink")
	expLatest = new(expvar.String)
)

func init() {
	exp.Set("latestevent", expLatest)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	camli.AddFlags()
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("usage: drink json.gz queue.db")
	}

	uploader := camli.NewUploader()

	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("Please set the env var GITHUB_TOKEN")
	}
	st := github.NewStarTracker(1000000000, os.Getenv("GITHUB_TOKEN"))
	exp.Set("github", st.Expvar())

	log.Println("Opening queue...")
	q, err := queue.Open(flag.Arg(1))
	fatalIfErr(err)
	defer func() { fatalIfErr(q.Close()) }()

	log.Println("Opening archive...")
	f, err := os.Open(flag.Arg(0))
	fatalIfErr(err)
	defer func() { fatalIfErr(f.Close()) }()
	r, err := github.NewTimelineArchiveReader(f)
	fatalIfErr(err)
	defer func() { fatalIfErr(r.Close()) }()

	log.Println("Reading events")
	for {
		var e github.Event
		if err := r.Read(&e); err == io.EOF {
			break
		} else {
			fatalIfErr(err)
		}

		exp.Add("readevents", 1)
		expLatest.Set(e.CreatedAt.String())

		switch e.Type {
		case "PushEvent":
			url := "https://github.com/" + e.Repo.Name + ".git"
			repo, err := uploader.GetRepo(url)
			fatalIfErr(err)
			if repo != nil {
				exp.Add("gotrepo", 1)
				exp.Add("queued", 1)
				q.Add(e.Repo.Name, repo.Parent)
				log.Printf("Queued repo %s", e.Repo.Name)
			}

			stars, parent, err := st.Get(e.Repo.Name)
			if github.Is404(err) {
				exp.Add("vanished", 1)
				log.Printf("Skipping repo %s (it vanished)", e.Repo.Name)
				continue
			}
			fatalIfErr(err)
			if stars < 10 {
				exp.Add("skipped", 1)
				log.Printf("Skipping repo %s (%d stars)", e.Repo.Name, stars)
				continue
			}

			exp.Add("queued", 1)
			q.Add(e.Repo.Name, parent)
			log.Printf("Queued repo %s", e.Repo.Name)

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
		}
	}
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
