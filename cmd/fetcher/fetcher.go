package main

import (
	"errors"
	"expvar"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"

	"github.com/thecodearchive/gitarchive/git"
	"github.com/thecodearchive/gitarchive/index"
	"github.com/thecodearchive/gitarchive/queue"
	"github.com/thecodearchive/gitarchive/weekmap"
)

var maxSize = MustGetenvInt("MAX_REPO_SIZE")

type Fetcher struct {
	q        *queue.Queue
	i        *index.Index
	bucket   *storage.BucketHandle
	schedule *weekmap.WeekMap

	exp *expvar.Map

	closing uint32
}

func (f *Fetcher) Run() error {
	f.exp.Set("fetchbytes", &expvar.Int{})
	for atomic.LoadUint32(&f.closing) == 0 {
		if !f.schedule.Get(time.Now()) {
			f.exp.Add("sleep", 1)
			interruptableSleep(5 * time.Minute)
			continue
		}

		name, parent, err := f.q.Pop()
		if err != nil {
			return err
		}

		if name == "" {
			f.exp.Add("emptyqueue", 1)
			interruptableSleep(30 * time.Second)
			continue
		}

		if err := f.Fetch(name, parent); err != nil {
			return err
		}
	}
	return nil
}

func (f *Fetcher) Fetch(name, parent string) error {
	f.exp.Add("fetches", 1)

	name = "github.com/" + name

	blacklistState, err := f.i.BlacklistState(name)
	if err != nil {
		return err
	}
	if blacklistState == index.Blacklisted {
		log.Println("[-] Skipping blacklisted repository.")
		f.exp.Add("blacklisted", 1)
		return nil
	}

	haves, deps, err := f.i.GetHaves(name)
	if err != nil {
		return err
	}

	if haves == nil {
		f.exp.Add("new", 1)
	}
	if parent != "" {
		f.exp.Add("forks", 1)
	}

	logVerb, logFork := "Cloning", ""
	if haves != nil {
		logVerb = "Fetching"
	}
	if parent != "" {
		logFork = fmt.Sprintf(" (fork of %s)", parent)
	}
	log.Printf("[+] %s %s%s...", logVerb, name, logFork)

	start := time.Now()
	bw := f.exp.Get("fetchbytes").(*expvar.Int)
	refs, packR, err := git.Fetch("git://"+name+".git", haves, os.Stderr, bw)
	if err, ok := err.(git.RemoteError); ok {
		if strings.Contains(err.Message, "Repository not found.") {
			log.Println("[-] Repository vanished :(")
			f.exp.Add("vanished", 1)
			return nil
		}
		if strings.Contains(err.Message, "DMCA") {
			log.Println("[-] Repository DMCA'd :(")
			f.exp.Add("dmca", 1)
			return nil
		}
	}
	if err != nil {
		return err
	}

	packRefName := fmt.Sprintf("%s/%d", name, time.Now().UnixNano())
	if packR != nil {
		w := f.bucket.Object(packRefName).NewWriter(context.Background())

		var r io.Reader = packR
		if blacklistState != index.Whitelisted {
			r = &io.LimitedReader{R: r, N: int64(maxSize)}
		}
		bytesFetched, err := io.Copy(w, r)
		if err != nil {
			return err
		}
		packR.Close()
		if r, ok := r.(*io.LimitedReader); ok && r.N <= 0 {
			w.CloseWithError(errors.New("too big"))
			f.i.AddBlacklist(name)
			log.Printf("[-] Repository too big :(")
			f.exp.Add("toobig", 1)
			return nil
		}
		w.Close()
		f.exp.Add("fetchtime", int64(time.Since(start)))
		log.Printf("[+] Got %d refs, %d bytes in %s.", len(refs), bytesFetched, time.Since(start))
	} else {
		// Empty packfile.
		packRefName = "EMPTY|" + packRefName
		f.exp.Add("emptypack", 1)
		log.Printf("[+] Got %d refs, and a empty packfile.", len(refs))
	}

	if parent != "" {
		parent = "github.com/" + parent
	}

	return f.i.AddFetch(name, parent, time.Now(), refs, packRefName, deps)
}

func (f *Fetcher) Stop() {
	atomic.StoreUint32(&f.closing, 1)
}

func interruptableSleep(d time.Duration) bool {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(c)
	t := time.NewTimer(d)
	select {
	case <-c:
		return false
	case <-t.C:
		return true
	}
}
