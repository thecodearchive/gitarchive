package main

import (
	"expvar"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

func main() {
	exp := expvar.NewMap("gitserver")

	//	err := metrics.StartInfluxExport(MustGetenv("INFLUX_ADDR"), "backpanel", exp)
	//	fatalIfErr(err)

	log.Println("[ ] Opening index...")
	//	i, err := index.Open(MustGetenv("DB_ADDR"))
	//	fatalIfErr(err)
	//	defer func() {
	//		log.Println("[ ] Closing index...")
	//		fatalIfErr(i.Close())
	//	}()

	s := &Server{exp: exp} //, i: i}
	http.Handle("/", s)

	/*c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("[ ] Stopping gracefully...")
		os.Exit
	}()*/

	fatalIfErr(http.ListenAndServe(":8080", nil))
}

func fatalIfErr(err error) {
	if err != nil {
		log.Printf("%+v", err)
		panic("fatal error") // panic to let the defer run
	}
}

func MustGetenv(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Panicln("Missing environment variable:", name)
	}
	return val
}

func OptGetenv(name, defaultVal string) string {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal
	}
	return val
}
