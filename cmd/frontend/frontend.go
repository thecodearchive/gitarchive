package main

import (
	"expvar"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/thecodearchive/gitarchive/index"
	"google.golang.org/cloud/storage"
)

type Frontend struct {
	i      *index.Index
	bucket *storage.BucketHandle

	exp *expvar.Map
}

const capabilities = "thin-pack side-band side-band-64k ofs-delta shallow agent=github.com/thecodearchive/gitarchive"

var testRefs = map[string]string{
	"refs/heads/master": "7ec915048d870617a6d497294923bb2262e0659e",
}

func (f *Frontend) Handle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(strings.TrimLeft(r.URL.Path, "/"),
		"api/v1/proxy/namespaces/default/services/frontend/")

	// /TIMESTAMP/github.com/USER/REPO/info/refs?service=git-upload-pack
	parts := strings.SplitN(path, "/", 5)
	if len(parts) != 5 {
		http.Error(w, "Unrecognized path", http.StatusNotFound)
		return
	}
	if parts[1] != "github.com" {
		http.Error(w, "Unrecognized repository", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Only GET supported", http.StatusNotImplemented)
		return
	}
	if parts[4] != "info/refs" {
		http.Error(w, "Unrecognized path", http.StatusNotFound)
		return
	}
	if r.URL.RawQuery != "service=git-upload-pack" {
		http.Error(w, "Unrecognized service", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")

	writePackLine(w, "# service=git-upload-pack")
	fmt.Fprint(w, "0000") // This is required. Don't ask why.
	firstLine := true
	for name, ref := range testRefs {
		extra := ""
		if firstLine {
			extra = "\x00" + capabilities
			firstLine = false
		}
		writePackLine(w, ref+" "+name+extra)
	}
	fmt.Fprint(w, "0000")

	log.Println("GET worked")
}

func writePackLine(w io.Writer, line string) {
	line = line + "\n"
	fmt.Fprintf(w, "%04x%s", len(line)+4, line)
}

func (f *Frontend) Run() error {
	srv := &http.Server{
		Addr:    ":80",
		Handler: http.HandlerFunc(f.Handle),
	}
	return srv.ListenAndServe()
}

func (f *Frontend) Stop() {}
