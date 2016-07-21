package main

import (
	"expvar"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"

	"github.com/thecodearchive/gitarchive/index"
	"github.com/thecodearchive/gitarchive/metrics"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	exp := expvar.NewMap("frontend")

	err := metrics.StartInfluxExport(MustGetenv("INFLUX_ADDR"), "frontend", exp)
	fatalIfErr(err)

	client, err := storage.NewClient(context.Background())
	fatalIfErr(err)

	bucket := client.Bucket(OptGetenv("FETCHER_BUCKET_NAME", "packfiles"))

	log.Println("[ ] Opening index...")
	i, err := index.Open(MustGetenv("DB_ADDR"))
	fatalIfErr(err)
	defer func() {
		log.Println("[ ] Closing index...")
		fatalIfErr(i.Close())
	}()

	f := &Frontend{exp: exp, i: i, bucket: bucket}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("[ ] Stopping gracefully...")
		f.Stop()
	}()

	fatalIfErr(f.Run())

	fmt.Print(exp.String())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Panic(err) // panic to let the defer run
	}
}

func MustGetenv(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Panicln("Missing environment variable:", name)
	}
	return val
}

func MustGetenvInt(name string) int {
	i, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		log.Panicln("Missing environment variable:", name)
	}
	return i
}

func OptGetenv(name, defaultVal string) string {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal
	}
	return val
}
