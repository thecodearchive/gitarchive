package main

import (
	"expvar"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/thecodearchive/gitarchive/github"
	"github.com/thecodearchive/gitarchive/index"
	"github.com/thecodearchive/gitarchive/metrics"
	"github.com/thecodearchive/gitarchive/queue"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	db, err := bolt.Open(MustGetenv("CACHE_PATH"), 0600, &bolt.Options{Timeout: 1 * time.Second})
	fatalIfErr(err)
	defer func() {
		log.Println("[ ] Closing cache...")
		if err := db.Sync(); err != nil {
			log.Println(err)
		}
		if err := db.Close(); err != nil {
			log.Println(err)
		}
	}()

	var t time.Time
	fatalIfErr(db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("gitarchive"))
		if b == nil {
			return nil
		}
		v := b.Get([]byte("_resume"))
		if v == nil {
			return nil
		}
		t, err = time.Parse(github.HourFormat, string(v))
		return err
	}))
	if t.IsZero() {
		t = time.Now().Truncate(time.Hour).Add(-12 * time.Hour)
		log.Println("[ ] Can't load resume file, starting 12 hours ago")
	} else {
		log.Printf("[+] Resuming from %s", t.Format(github.HourFormat))
	}

	exp := expvar.NewMap("drinker")
	expEvents := new(expvar.Map).Init()
	expLatest := new(expvar.String)
	exp.Set("latestevent", expLatest)
	exp.Set("events", expEvents)

	err = metrics.StartInfluxExport(MustGetenv("INFLUX_ADDR"), "drinker", exp)
	fatalIfErr(err)

	log.Println("[ ] Opening queue...")
	q, err := queue.Open(MustGetenv("DB_ADDR"))
	fatalIfErr(err)
	defer func() {
		log.Println("[ ] Closing queue...")
		if err := q.Close(); err != nil {
			log.Println(err)
		}
	}()

	log.Println("[ ] Opening index...")
	i, err := index.Open(MustGetenv("DB_ADDR"))
	fatalIfErr(err)
	defer func() {
		log.Println("[ ] Closing index...")
		fatalIfErr(i.Close())
	}()

	st := github.NewStarTracker(db, MustGetenv("GITHUB_TOKEN"))
	exp.Set("github", st.Expvar())

	// Set NoSync since we don't care about losing data since the last sync,
	// which we manually do when making a checkpoint.
	db.NoSync = true

	d := &Drinker{
		q: q, st: st, i: i,
		exp: exp, expEvents: expEvents, expLatest: expLatest,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("[ ] Stopping gracefully...")
		d.Stop()
	}()

	startTime := t.Add(time.Hour).Add(2 * time.Minute)
	for {
		if time.Now().Before(startTime) {
			log.Printf("[ ] Waiting for the %s archive until %s...",
				t.Format(github.HourFormat), startTime)
			if !interruptableSleep(startTime.Sub(time.Now())) {
				break
			}
		}
		log.Printf("[ ] Opening %s archive download...", t.Format(github.HourFormat))
		a, err := github.DownloadArchive(t)
		if err != nil {
			log.Println("[-] Failed to download archive:", err)
			break
		}
		if a == nil {
			exp.Add("archives404", 1)
			startTime = time.Now().Add(2 * time.Minute)
			continue
		}

		log.Printf("[+] Archive %s found, consuming...", t.Format(github.HourFormat))
		err = d.DrinkArchive(a)
		a.Close()
		if err == StoppedError {
			break
		}
		if err != nil {
			log.Println("[-] Failed to drink archive:", err)
			break
		}

		exp.Add("archivesfinished", 1)
		t = t.Add(time.Hour)
		startTime = t.Add(time.Hour).Add(2 * time.Minute)

		if err := db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte("gitarchive"))
			if err != nil {
				return err
			}
			return b.Put([]byte("_resume"), []byte(t.Format(github.HourFormat)))
		}); err != nil {
			log.Println("[-] Failed to save checkpoint:", err)
			break
		}
		if err := db.Sync(); err != nil {
			log.Println("[-] Failed to sync the database:", err)
			break
		}
	}

	log.Println("[+] Processed events until", expLatest)
	fmt.Print(exp.String())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func MustGetenv(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Fatalln("Missing environment variable:", name)
	}
	return val
}
