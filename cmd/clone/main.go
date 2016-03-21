package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/git"
)

func main() {
	url := os.Args[1]

	uploader := camli.NewUploader()

	refs, caps, err := git.Clone(url, uploader, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
	x, _ := json.Marshal(caps)
	fmt.Fprintf(os.Stderr, "%s\n", x)
	x, _ = json.Marshal(refs)
	fmt.Fprintf(os.Stderr, "%s\n", x)

	err = uploader.PutRepo(&camli.Repo{
		Name:      url,
		Retrieved: time.Now(),
		Refs:      refs,
	})
	if err != nil {
		log.Fatal(err)
	}
}
