package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/thecodearchive/gitarchive/git"
)

func main() {
	objs, refs, caps, err := git.Clone(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	x, _ := json.Marshal(caps)
	fmt.Fprintf(os.Stderr, "%s\n", x)
	x, _ = json.Marshal(refs)
	fmt.Fprintf(os.Stderr, "%s\n", x)
	fmt.Println("Received objects:", len(objs))
}
