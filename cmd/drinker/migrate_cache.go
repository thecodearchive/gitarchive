//+build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/thecodearchive/gitarchive/github"
)

type entry struct {
	Key   string
	Value github.Repo
}

func main() {
	db, err := bolt.Open(os.Args[1], 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte("StarTracker"))
		if err != nil {
			return err
		}
		b, err := tx.CreateBucket([]byte("gitarchive"))
		if err != nil {
			return err
		}
		return b.Put([]byte("_resume"), []byte(os.Args[2]))
	}); err != nil {
		log.Fatal(err)
	}

	dec := json.NewDecoder(os.Stdin)
	var n int
	tx, err := db.Begin(true)
	if err != nil {
		log.Fatal(err)
	}
	b := tx.Bucket([]byte("StarTracker"))
	for {
		var e entry
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		if e.Key == "" || e.Value.LastUpdated.IsZero() {
			log.Fatal(e)
		}

		v, err := e.Value.MarshalMsg(nil)
		if err != nil {
			log.Fatal(err)
		}
		if err := b.Put([]byte(e.Key), v); err != nil {
			log.Fatal(err)
		}

		n++
		if n%1000 == 0 {
			if err := tx.Commit(); err != nil {
				log.Fatal(err)
			}
			tx, err = db.Begin(true)
			if err != nil {
				log.Fatal(err)
			}
			b = tx.Bucket([]byte("StarTracker"))
			fmt.Fprintf(os.Stderr, "\r%d", n)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "\r%d\n", n)
}
