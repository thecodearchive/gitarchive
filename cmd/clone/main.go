package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/thecodearchive/gitarchive/git"
)

func main() {
	url := os.Args[1]

	refs, rc, err := git.Fetch(url, nil, os.Stderr, nil)
	if err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(refs)

	if rc == nil {
		log.Println("Empty packfile.")
		return
	}

	n, err := io.Copy(ioutil.Discard, rc)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Fetched %d bytes.", n)
}
