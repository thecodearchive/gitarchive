package camli

import (
	"testing"
	"time"
)

// These tests require a local running instance of Camlistore.
// `devam server -wipe` should work.

var uploader = NewUploader()

func TestPutSomeBlobs(t *testing.T) {
	objects := []string{
		"hello", // aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
		"just",  // d95b79cfc988b3b165ceb830a9c8932d1b52cf18
		"testing this",
	}

	for _, o := range objects {
		err := uploader.PutObject([]byte(o))
		if err != nil {
			t.Errorf("Couldn't put an object: %v", err)
		}
	}

	err := uploader.PutRepo(&Repo{
		Name:      "github.com/some/test",
		Retrieved: time.Date(2009, time.April, 10, 23, 0, 0, 0, time.UTC),
		Refs: map[string]string{
			"master": "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d",
		},
	})
	if err != nil {
		t.Errorf("Couldn't put a repo: %v", err)
	}
}
