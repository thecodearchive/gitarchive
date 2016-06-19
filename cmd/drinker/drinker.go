package main

import (
	"encoding/json"
	"errors"
	"expvar"
	"io"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/thecodearchive/gitarchive/github"
	"github.com/thecodearchive/gitarchive/index"
	"github.com/thecodearchive/gitarchive/queue"
)

type Drinker struct {
	q  *queue.Queue
	i  *index.Index
	st *github.StarTracker

	exp       *expvar.Map
	expEvents *expvar.Map
	expLatest *expvar.String

	closing uint32
}

func (d *Drinker) DrinkArchive(a io.Reader) error {
	r, err := github.NewTimelineArchiveReader(a)
	if err != nil {
		return err
	}
	defer r.Close()

	for atomic.LoadUint32(&d.closing) == 0 {
		var e github.Event
		if err := r.Read(&e); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		d.expEvents.Add(e.Type, 1)
		d.expLatest.Set(e.CreatedAt.String())

		switch e.Type {
		case "PushEvent":
			d.handlePushEvent(&e)

		case "CreateEvent":
			var ce github.CreateEvent
			if err := json.Unmarshal(e.Payload, &ce); err != nil {
				d.exp.Add("dropped", 1)
				log.Printf("[-] CreateEvent Unmarshal error: %s; dropped event: %#v", err, e)
				continue
			}
			switch ce.RefType {
			case "repository":
				d.st.CreateEvent(e.Repo.Name, "", e.CreatedAt.Time)
			case "branch", "tag":
				// TODO
			}

		case "WatchEvent":
			d.st.WatchEvent(e.Repo.Name, e.CreatedAt.Time)

		case "ForkEvent":
			var fe github.ForkEvent
			if err := json.Unmarshal(e.Payload, &fe); err != nil {
				d.exp.Add("dropped", 1)
				log.Printf("[-] ForkEvent Unmarshal error: %s; dropped event: %#v", err, e)
				continue
			}
			d.st.CreateEvent(fe.Forkee.FullName, e.Repo.Name, e.CreatedAt.Time)

		case "DeleteEvent":
			// TODO

		case "PublicEvent":
			d.st.CreateEvent(e.Repo.Name, "", e.CreatedAt.Time)
		}
	}

	if atomic.LoadUint32(&d.closing) == 1 {
		return StoppedError
	}

	return nil
}

func (d *Drinker) handlePushEvent(e *github.Event) {
	latestFetch, err := d.i.GetLatest("github.com/" + e.Repo.Name)
	if err != nil {
		log.Println("[-] Index error:", err)
	} else {
		if !latestFetch.IsZero() && e.CreatedAt.Time.Before(latestFetch) {
			d.exp.Add("alreadyfetched", 1)
			return
		}
	}

	stars, parent, err := d.st.Get(e.Repo.Name)
	if rate := github.IsRateLimit(err); rate != nil {
		d.exp.Add("ratehits", 1)
		log.Println("[-] Hit GitHub ratelimits, sleeping until", rate.Reset)
		if interruptableSleep(rate.Reset.Sub(time.Now()) + 1*time.Minute) {
			log.Println("[+] Resuming...")
			d.handlePushEvent(e)
		}
		return
	}
	if github.Is404(err) {
		d.exp.Add("vanished", 1)
		return
	}
	if err != nil {
		d.exp.Add("dropped", 1)
		log.Printf("[-] ST error: %s; dropped event: %#v", err, e)
		return
	}
	if stars < 10 {
		d.exp.Add("skipped", 1)
		return
	}

	d.exp.Add("queued", 1)
	d.q.Add(e.Repo.Name, parent)
}

var StoppedError = errors.New("process was asked to gracefully stop")

func (d *Drinker) Stop() {
	atomic.StoreUint32(&d.closing, 1)
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
