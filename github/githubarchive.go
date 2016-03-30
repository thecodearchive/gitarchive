package github

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/djherbis/buffer"
	"github.com/djherbis/nio"
	"github.com/google/go-github/github"
)

type Event struct {
	CreatedAt github.Timestamp `json:"created_at"`
	Payload   json.RawMessage  `json:"payload"`
	Repo      struct {
		Name string `json:"name"`
	} `json:"repo"`
	Type string `json:"type"`
}

type CreateEvent struct {
	RefType string `json:"ref_type"`
}

type ForkEvent struct {
	Forkee struct {
		FullName string `json:"full_name"`
	} `json:"forkee"`
}

// TimelineArchiveReader reads a .json.gz like those offered by githubarchive.org
type TimelineArchiveReader struct {
	jr *json.Decoder
	z  *gzip.Reader
}

func NewTimelineArchiveReader(r io.Reader) (*TimelineArchiveReader, error) {
	z, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &TimelineArchiveReader{jr: json.NewDecoder(z), z: z}, nil
}

func (t *TimelineArchiveReader) Read(e *Event) error {
	return t.jr.Decode(e)
}

func (t *TimelineArchiveReader) Close() error {
	return t.z.Close()
}

const HourFormat = "2006-01-02-15"

func DownloadArchive(t time.Time) (io.ReadCloser, error) {
	hc := &http.Client{
		Transport: &http.Transport{
			Dial:                (&net.Dialer{Timeout: 30 * time.Second}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     &tls.Config{ServerName: "storage.googleapis.com"},
		},
	}

	hour := t.Format(HourFormat)
	r, err := hc.Get("https://data.githubarchive.org/" + hour + ".json.gz")
	if err != nil {
		return nil, err
	}
	if r.StatusCode == http.StatusNotFound {
		io.Copy(ioutil.Discard, r.Body)
		r.Body.Close()
		return nil, nil
	}
	if r.StatusCode != http.StatusOK {
		r.Body.Close()
		return nil, fmt.Errorf("HTTP error downloading %s: %v", hour, r.Status)
	}

	// Concurrently download the archive to a memory buffer of 1MB chunks
	buf := buffer.NewPartition(buffer.NewMemPool(1024 * 1024))
	pr, pw := nio.Pipe(buf)
	go func() {
		_, err := io.Copy(pw, r.Body)
		pw.CloseWithError(err)
		r.Body.Close()
	}()
	return pr, nil
}
