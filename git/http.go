package git

import (
	"bytes"
	"expvar"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// Fetch fetches the git repo at gitURL proving the refs in haves as
// haves.
//
// Sideband messages from the git server are writetn to msgW, and the
// number of bytes fetched is incremented in bwCounter. bwCounter is
// incremented here to get fine-grained metrics.
func Fetch(gitURL string, haves map[string]struct{}, msgW io.Writer,
	bwCounter *expvar.Int) (refs map[string]string, r io.ReadCloser, err error) {

	req, err := http.NewRequest("GET", gitURL+"/info/refs?service=git-upload-pack", nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "github.com/thecodearchive/gitarchive/git")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("GET /info/refs: %d", resp.StatusCode)
	}
	refs, err = ParseSmartResponse(resp.Body)
	if err != nil {
		return
	}

	for name := range refs {
		if strings.HasPrefix(name, "refs/pull/") {
			delete(refs, name)
		}
	}

	var wants []string
	for name, ref := range refs {
		if _, ok := haves[ref]; ok {
			continue
		}
		wants = append(wants, refs[name])
	}
	sort.Strings(wants)

	if len(wants) == 0 {
		return
	}

	body := &bytes.Buffer{}
	last := ""
	for _, want := range wants {
		if last == want {
			continue
		}
		command := "want " + want
		if last == "" {
			command += " ofs-delta side-band-64k thin-pack"
		}
		command += "\n"
		body.WriteString(fmt.Sprintf("%04x%s", len(command)+4, command))
		last = want
	}
	body.WriteString("0000")
	for have := range haves { // TODO: sort the haves
		command := "have " + have + "\n"
		body.WriteString(fmt.Sprintf("%04x%s", len(command)+4, command))
	}
	body.WriteString("0009done\n")

	req, err = http.NewRequest("POST", gitURL+"/git-upload-pack", body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	req.Header.Set("User-Agent", "github.com/thecodearchive/gitarchive/git")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("POST /git-upload-pack: %d", resp.StatusCode)
	}

	sbr := &sideBandReader{Upstream: resp.Body, MsgW: msgW}
	cr := &countingReader{Upstream: sbr, Counter: bwCounter}

	// Peek into the first 32 bytes to make sure it's not an empty
	// packfile.
	// Needs to be > 32 bytes (hdr is 12, trailer is 20).
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, cr, 64)
	if err == io.EOF && n == 32 {
		resp.Body.Close()
		return refs, nil, nil
	}
	if err != nil {
		resp.Body.Close()
		return
	}
	rc := struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(&buf, cr),
		Closer: resp.Body,
	}
	return refs, rc, nil
}
