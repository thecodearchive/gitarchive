package main

import (
	"io"
	"time"
)

var (
	weekday = [...]int{
		1, 1, 10, 30, 70, 150, 250, 250, 250, 250, 250, 250, // AM
		150, 70, 30, 10, 1, 1, 1, 1, 1, 1, 1, 1, // PM
	}
	saturday = [...]int{
		1, 1, 10, 30, 70, 150, 250, 250, 250, 250, 250, 250, // AM
		250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, // PM
	}
	sunday = [...]int{
		250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, // AM
		250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, // PM
	}
	monday = [...]int{
		250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, 250, // AM
		150, 70, 30, 10, 1, 1, 1, 1, 1, 1, 1, 1, // PM
	}

	// Simple weekly timetable of download speed caps in MB/s.
	timetable = [7][24]int{sunday, monday, weekday, weekday, weekday, weekday, saturday}
)

type ChokedReader struct {
	r    io.Reader
	size int
	n    int
	t    *time.Ticker
}

// ChokeReader is a conveniece function that returns a ChokedReader which
// limits bandwidth consumption to mbps MB/s, down to 1 millisecond precision.
func ChokeReader(r io.ReadCloser, mbps int) *ChokedReader {
	return &ChokedReader{
		r:    r,
		size: mbps << 10, // 1 MB/s == 1 KB/ms.
		t:    time.NewTicker(time.Millisecond),
	}
}

func (cr *ChokedReader) Close() error {
	cr.t.Stop()
	return nil
}

func (cr *ChokedReader) Read(buf []byte) (n int, err error) {
	chunk := cr.size
	for err == nil && len(buf) > 0 {
		if chunk > len(buf) {
			chunk = len(buf)
		}
		var m int
		m, err = cr.r.Read(buf[:chunk])
		cr.n += m
		n += m
		buf = buf[m:]
		if cr.n > cr.size {
			<-cr.t.C
			cr.n = 0
		}
	}
	return
}
