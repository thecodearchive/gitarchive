package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"go4.org/types"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/git"
)

func main() {
	camli.AddFlags()
	flag.Parse()
	url := flag.Arg(0)

	uploader := camli.NewUploader()

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

	res, err := git.Fetch(url, haves, uploader, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(res)

	err = uploader.PutRepo(&camli.Repo{
		Name:      url,
		Retrieved: types.Time3339(time.Now()),
		Refs:      res.Refs,
	})
	if err != nil {
		log.Fatal(err)
	}
}
