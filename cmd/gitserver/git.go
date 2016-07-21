package main

import (
	"expvar"
	"net/http"
	"strings"
	"time"

	"camlistore.org/pkg/index"
)

type Server struct {
	i   *index.Index
	exp *expvar.Map

	closing uint32
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Valid pull paths are like:
	//   - http://codearchive.0f.io/latest/github.com/FiloSottle/gvt
	//   - http://codearchive.0f.io/2016-07-03/github.com/FiloSottle/gvt
	parts := strings.Split(req.URL.Path[1:], "/")
	if len(parts) < 5 {
		http.Error(rw, "not found", http.StatusNotFound)
		return
	}

	repo := parts[1] + "/" + parts[2] + "/" + parts[3]
	timestamp := time.Now()
	var err error
	if parts[0] != "latest" {
		// TODO think of a better date format.
		timestamp, err = time.Parse("2006-01-02", parts[0])
		if err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
		}
	}

	// TODO check repo is in index here.
	rest := strings.Join(parts[4:], "/")
	queryvals := req.URL.Query()
	if rest == "info/refs" && queryvals.Get("service") == "git-upload-pack" {
		s.handleRefs(repo, timestamp, rw)
		return
	}

	if rest == "git-upload-pack" {
		s.handlePackfiles(repo, timestamp, rw)
		return
	}

	http.Error(rw, "not found", http.StatusBadRequest)
}

func (s *Server) handleRefs(repo string, timestamp time.Time, rw http.ResponseWriter) {

}

func (s *Server) handlePackfiles(repo string, timestamp time.Time, rw http.ResponseWriter) {
	http.Error(rw, "not implmented", http.StatusNotImplemented)
}
