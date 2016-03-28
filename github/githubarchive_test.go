package github

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"testing"
	"time"
)

func TestTimelineArchiveReader(t *testing.T) {
	f, err := os.Open("testdata/2016-03-25-12.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := NewTimelineArchiveReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	count := 0
	for {
		var e Event
		if err := r.Read(&e); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		switch e.Type {
		case "PushEvent":
			t.Log(e.Repo.Name)
			count += 1
		}
	}
	if count != 18629 {
		t.Error("Wrong count", count)
	}
}

func TestDownloadArchive(t *testing.T) {
	a, err := DownloadArchive(time.Date(2015, time.September, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s := sha256.New()
	io.Copy(s, a)
	if hex.EncodeToString(s.Sum(nil)) != "bd2e53db12d4b4d8a6d55adbccaf13891fcd861009689039e12e0e1593fceff0" {
		t.Error(hex.EncodeToString(s.Sum(nil)))
	}
}

func TestDownloadArchiveMissing(t *testing.T) {
	a, err := DownloadArchive(time.Date(2020, time.September, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if a != nil {
		t.Fatal("Time travel!", a)
	}
}
