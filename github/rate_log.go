package github

import (
	"flag"
	"log"
	"net/http"
	"os"
)

// This file contains extra debugging code for a GitHub ratelimit bug
// Delete when it yielded results

var (
	ghLogFile     = flag.String("github", "./github.txt", "path to the GH ratelimit headers log")
	lastRateReset string
)

func logGHRateReset(r *http.Response) {
	if r.Header.Get("X-RateLimit-Reset") == lastRateReset {
		return
	}

	f, err := os.OpenFile(*ghLogFile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0664)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	r.Request.Write(f)
	r.Write(f)
	lastRateReset = r.Header.Get("X-RateLimit-Reset")
}
