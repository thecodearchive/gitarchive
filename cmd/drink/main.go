package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/github"
)

func main() {
	camli.AddFlags()
	flag.Parse()
	path := flag.Arg(0)

	uploader := camli.NewUploader()

	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("Please set the env var GITHUB_TOKEN")
	}
	st := github.NewStarTracker(1000000000, os.Getenv("GITHUB_TOKEN"))

	f, err := os.Open(path)
	fatalIfErr(err)
	defer f.Close()
	r, err := github.NewTimelineArchiveReader(f)
	fatalIfErr(err)
	defer r.Close()
	for {
		var e github.Event
		if err := r.Read(&e); err == io.EOF {
			break
		} else {
			fatalIfErr(err)
		}

		switch e.Type {
		case "PushEvent":
			url := "https://github.com/" + e.Repo.Name + ".git"
			repo, err := uploader.GetRepo(url)
			fatalIfErr(err)
			if repo != nil {
				continue // TODO
			}

			stars, parent, err := st.Get(e.Repo.Name)
			if github.Is404(err) {
				log.Printf("Skipping repo %s (it vanished)", e.Repo.Name)
				continue
			}
			fatalIfErr(err)
			if stars < 10 {
				log.Printf("Skipping repo %s (%d stars)", e.Repo.Name, stars)
				continue
			}

			fmt.Println("QUEUE", e.Repo.Name, parent)

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
