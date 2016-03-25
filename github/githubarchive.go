package github

import (
	"compress/gzip"
	"encoding/json"
	"io"
)

type Event struct {
	CreatedAt string          `json:"created_at"`
	Payload   json.RawMessage `json:"payload"`
	Repo      struct {
		Name string `json:"name"`
	} `json:"repo"`
	Type string `json:"type"`
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
