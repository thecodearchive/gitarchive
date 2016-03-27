package github

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func checkStars(t *testing.T, st *StarTracker, name string, starsWant int) {
	starsNew, _, err := st.Get("FiloSottile/ansible-sshknownhosts")
	if err != nil {
		t.Fatal(err)
	}
	if starsNew != starsWant {
		t.Fatal(starsNew, starsWant)
	}
}

func TestStarTracker(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Fatal("Please set the env var GITHUB_TOKEN")
	}
	st := NewStarTracker(100, os.Getenv("GITHUB_TOKEN"))
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

	b := &bytes.Buffer{}
	if err := st.SaveCache(b); err != nil {
		t.Fatal(err)
	}
	st.WatchEvent("FiloSottile/ansible-sshknownhosts", time.Now().Add(2*time.Hour))
	checkStars(t, st, "FiloSottile/ansible-sshknownhosts", starsOld+2)
	if err := st.LoadCache(b); err != nil {
		t.Fatal(err)
	}
	checkStars(t, st, "FiloSottile/ansible-sshknownhosts", starsOld+1)
}
