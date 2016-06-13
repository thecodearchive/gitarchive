package main

import (
	"expvar"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"

	"github.com/thecodearchive/gitarchive/camli"
	"github.com/thecodearchive/gitarchive/git"
	"github.com/thecodearchive/gitarchive/queue"
	"github.com/thecodearchive/gitarchive/weekmap"
)

type Fetcher struct {
	q        *queue.Queue
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

	url := "https://github.com/" + name + ".git"
	repo, err := &camli.Repo{}, error(nil) // f.u.GetRepo(url)
	if err != nil {
		return err
	}

	haves := make(map[string]struct{})
	var packfiles []string
	if repo != nil {
		for _, have := range repo.Refs {
			haves[have] = struct{}{}
		}
		packfiles = repo.Packfiles
	} else {
		f.exp.Add("new", 1)
	}

	// On first clone of a fork, import all parent's refs and packs.
	// TODO: we might want to experiment with always merging refs and packs.
	// Smaller and faster fetches, but possibly a lot of waste in serving clones.
	if parent != "" && repo == nil {
		f.exp.Add("forks", 1)
		//mainURL := "https://github.com/" + parent + ".git"
		mainRepo, err := repo, error(nil) //f.u.GetRepo(mainURL) //TODO
		if err != nil {
			return err
		}
		if mainRepo != nil {
			for _, have := range mainRepo.Refs {
				haves[have] = struct{}{}
			}
			packfiles = mainRepo.Packfiles
		}
	}

	logVerb, logFork := "Cloning", ""
	if repo != nil {
		logVerb = "Fetching"
	}
	if parent != "" {
		logFork = fmt.Sprintf(" (fork of %s)", parent)
	}
	log.Printf("[+] %s %s%s...", logVerb, name, logFork)

	packrefname := fmt.Sprintf("%s_%d", name, time.Now().Unix())
	w := f.bucket.Object(packrefname).NewWriter(context.Background())

	start := time.Now()
	bw := f.exp.Get("fetchbytes").(*expvar.Int)
	refs, r, err := git.Fetch(url, haves, os.Stderr, bw)
	if err != nil {
		return err
	}
	bytesFetched, err := io.Copy(w, r)
	if err != nil {
		return err
	}
	w.Close()
	f.exp.Add("fetchtime", int64(time.Since(start)))

	//	f.exp.Add("emptypack", 1)
	packfiles = append(packfiles, packrefname)

	// TODO
	/*err = f.u.PutRepo(&camli.Repo{
		Name:      url,
		Retrieved: time.Now(),
		Refs:      refs,
		Packfiles: packfiles,
	})*/
	log.Printf("[+] Got %d refs, %d bytes.", len(refs), bytesFetched)
	return err
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
