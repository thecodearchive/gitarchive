package github

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boltdb/bolt"
)

func checkStars(t *testing.T, st *StarTracker, name string, starsWant int) {
	starsNew, _, err := st.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	if starsNew != starsWant {
		t.Fatal(name, starsNew, starsWant)
	}
}

func TestStarTracker(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Fatal("Please set the env var GITHUB_TOKEN")
	}
	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	db, err := bolt.Open(filepath.Join(dir, "my.db"), 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	st := NewStarTracker(db, os.Getenv("GITHUB_TOKEN"))
	starsOld, parent, err := st.Get("FiloSottile/ansible-sshknownhosts")
	if err != nil {
		t.Fatal(err)
	}
	if parent != "bfmartin/ansible-sshknownhosts" {
		t.Fatal(parent)
	}
	st.panicIfNetwork = true
	time.Sleep(1)
	st.WatchEvent("FiloSottile/ansible-sshknownhosts", time.Now().Add(-1*time.Hour))
	checkStars(t, st, "FiloSottile/ansible-sshknownhosts", starsOld)
	st.WatchEvent("FiloSottile/ansible-sshknownhosts", time.Now().Add(time.Hour))
	checkStars(t, st, "FiloSottile/ansible-sshknownhosts", starsOld+1)

	st.CreateEvent("FiloSottile/foo", "", time.Now())
	checkStars(t, st, "FiloSottile/foo", 0)
}
