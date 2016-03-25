package github

import (
	"os"
	"testing"
	"time"
)

func TestStarTracker(t *testing.T) {
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
	st.WatchEvent("FiloSottile/ansible-sshknownhosts", time.Now().Add(time.Hour))
	starsNew, _, err := st.Get("FiloSottile/ansible-sshknownhosts")
	if err != nil {
		t.Fatal(err)
	}
	if starsNew != starsOld+1 {
		t.Fatal(starsOld, starsNew)
	}
	st.WatchEvent("FiloSottile/ansible-sshknownhosts", time.Now().Add(-1*time.Hour))
	starsNew, _, err = st.Get("FiloSottile/ansible-sshknownhosts")
	if err != nil {
		t.Fatal(err)
	}
	if starsNew != starsOld+1 {
		t.Fatal(starsOld, starsNew)
	}
}
