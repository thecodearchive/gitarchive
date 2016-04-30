//+build ignore

package main

import (
	"bufio"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/thecodearchive/gitarchive/weekmap"
)

func main() {
	weekmap := (*weekmap.WeekMap)(&big.Int{})
	scanner := bufio.NewScanner(os.Stdin)
	for wd := time.Weekday(0); wd < 7; wd++ {
		fmt.Printf("%s: ", wd)
		if !scanner.Scan() {
			log.Fatal("\nI wanted data!")
		}
		input := scanner.Text()
		if len(input) != 24 {
			log.Fatal("Wrong length.")
		}
		for h := 0; h < len(input); h++ {
			if input[h] == '1' {
				weekmap.Set(wd, h, true)
			} else if input[h] != '0' {
				log.Fatal("Unrecognized char.")
			}
		}
	}
	fmt.Println(weekmap.Pack())
}
