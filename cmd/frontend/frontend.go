package main

import (
	"expvar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
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
	"HEAD":              "7ec915048d870617a6d497294923bb2262e0659e",
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
	repo := strings.Join(parts[1:4], "/")
	timestamp := parts[0]

	if r.Method == "GET" {
		f.FetchRefs(w, r, timestamp, repo, parts[4])
	} else if r.Method == "POST" {
		f.PostObjects(w, r, timestamp, repo, parts[4])
	} else {
		http.Error(w, "Only GET supported", http.StatusNotImplemented)
	}

	return
}

func (f *Frontend) FetchRefs(w http.ResponseWriter, r *http.Request, timestamp, repo, extra string) {
	if extra != "info/refs" {
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

func (f *Frontend) PostObjects(w http.ResponseWriter, r *http.Request, timestamp, repo, extra string) {
	if extra != "git-upload-pack" {
		http.Error(w, "Unrecognized path", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")

	gitR, cmd, err := runGitUploadPack(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(w, io.TeeReader(gitR, os.Stdout))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd.Wait()
}

func writePackLine(w io.Writer, line string) {
	line = line + "\n"
	fmt.Fprintf(w, "%04x%s", len(line)+4, line)
}

func runGitUploadPack(r io.Reader) (io.Reader, *exec.Cmd, error) {
	cmd := exec.Command("git-upload-pack", "/testrepo")
	cmd.Stdin = r
	pr, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	// Discard the advertisement
	for {
		pktLenHex := make([]byte, 4)
		if _, err := io.ReadFull(pr, pktLenHex); err == io.EOF {
			return nil, nil, io.ErrUnexpectedEOF
		} else if err != nil {
			return nil, nil, err
		}
		pktLen, err := strconv.ParseUint(string(pktLenHex), 16, 16)
		if err != nil {
			return nil, nil, err
		}

		// "0000" marker
		if pktLen == 0 {
			break
		}

		if _, err := io.CopyN(ioutil.Discard, pr, int64(pktLen-4)); err != nil {
			return nil, nil, err
		}
	}

	return pr, cmd, nil
}

func (f *Frontend) Run() error {
	srv := &http.Server{
		Addr:    ":80",
		Handler: http.HandlerFunc(f.Handle),
	}
	return srv.ListenAndServe()
}

func (f *Frontend) Stop() {}
