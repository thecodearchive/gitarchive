package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/git"
)

func main() {
	camli.AddFlags()
	flag.Parse()
	url := flag.Arg(0)

	uploader, err := camli.NewUploader()
	if err != nil {
		log.Fatal(err)
	}

	repo, err := uploader.GetRepo(url)
	if err != nil {
		log.Fatal(err)
	}

	haves := make(map[string]struct{})
	if repo != nil {
		for _, have := range repo.Refs {
			haves[have] = struct{}{}
		}
	}

	var packfiles []string
	if repo != nil {
		packfiles = repo.Packfiles
	}

	res, err := git.Fetch(url, haves, uploader, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(res)

	if res.PackRef != "" {
		packfiles = append(packfiles, res.PackRef)
	}
	err = uploader.PutRepo(&camli.Repo{
		Name:      url,
		Retrieved: time.Now(),
		Refs:      res.Refs,
		Packfiles: packfiles,
	})
	if err != nil {
		log.Fatal(err)
	}
}
