package github

import (
	"io"
	"os"
	"testing"
)

func TestTimelineArchiveReader(t *testing.T) {
	f, err := os.Open("testdata/2016-03-25-12.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := NewTimelineArchiveReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	count := 0
	for {
		var e Event
		if err := r.Read(&e); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		switch e.Type {
		case "PushEvent":
			t.Log(e.Repo.Name)
			count += 1
		}
	}
	if count != 18629 {
		t.Error("Wrong count", count)
	}
}
