package queue

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func fatalIfErr(t *testing.T, err error) {
	if err != nil {
		panic(err)
	}
}

func checkNAndP(t *testing.T, wantN, wantP, n, p string) {
	if n != wantN {
		t.Fatalf("Wanted n = %s, got %s", wantN, n)
	}
	if p != wantP {
		t.Fatalf("Wanted p = %s, got %s", wantP, p)
	}
}

func TestQueue(t *testing.T) {
	os.Remove("./test.db")
	q, err := Open("sqlite3", "./test.db")
	fatalIfErr(t, err)
	testQueue(t, q)
	fatalIfErr(t, os.Remove("./test.db"))
}

func TestQueueMySQL(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" {
		t.Skip("TEST_MYSQL_DSN missing, skipping MySQL test")
	}
	q, err := Open("mysql", os.Getenv("TEST_MYSQL_DSN"))
	fatalIfErr(t, err)
	testQueue(t, q)
}

func testQueue(t *testing.T, q *Queue) {
	n, p, err := q.Pop()
	fatalIfErr(t, err)
	checkNAndP(t, "", "", n, p)
	fatalIfErr(t, q.Add("a", ""))
	fatalIfErr(t, q.Add("b", "b"))
	fatalIfErr(t, q.Add("a", ""))
	n, p, err = q.Pop()
	fatalIfErr(t, err)
	checkNAndP(t, "a", "", n, p)
	fatalIfErr(t, q.Add("c", "c"))
	n, p, err = q.Pop()
	fatalIfErr(t, err)
	checkNAndP(t, "b", "b", n, p)
	n, p, err = q.Pop()
	fatalIfErr(t, err)
	checkNAndP(t, "c", "c", n, p)
	n, p, err = q.Pop()
	fatalIfErr(t, err)
	checkNAndP(t, "", "", n, p)
	fatalIfErr(t, q.Close())
}

func waitForValue(t *testing.T, q *Queue, wantN, wantP string) {
	var n, p string
	var err error
	for i := 0; i < 500; i++ {
		n, p, err = q.Pop()
		fatalIfErr(t, err)
		if n == wantN && p == wantP {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("Never got to n = %s", wantN)
}

func TestQueueConcurrency(t *testing.T) {
	if os.Getenv("BE_ADDER") == "1" {
		q, err := Open("sqlite3", "./test_concurr.db")
		fatalIfErr(t, err)
		for {
			fatalIfErr(t, q.Add("a", ""))
			time.Sleep(5 * time.Millisecond)
		}
	}

	os.Remove("./test_concurr.db")
	q, err := Open("sqlite3", "./test_concurr.db")
	fatalIfErr(t, err)

	n, p, err := q.Pop()
	fatalIfErr(t, err)
	checkNAndP(t, "", "", n, p)

	cmd := exec.Command(os.Args[0], "-test.run=^TestQueueConcurrency$")
	cmd.Env = append(os.Environ(), "BE_ADDER=1")
	fatalIfErr(t, cmd.Start())

	waitForValue(t, q, "a", "")
	waitForValue(t, q, "", "")
	waitForValue(t, q, "a", "")
	waitForValue(t, q, "a", "")
	waitForValue(t, q, "", "")

	fatalIfErr(t, cmd.Process.Signal(os.Interrupt))
	err = cmd.Wait()
	fatalIfErr(t, q.Close())
	fatalIfErr(t, os.Remove("./test_concurr.db"))
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		if !e.Sys().(syscall.WaitStatus).Signaled() {
			t.Fatalf("It died by itself: %s", e.Stderr)
		}
	} else {
		t.Fatal("How the hell did it exit cleanly?")
	}
}
