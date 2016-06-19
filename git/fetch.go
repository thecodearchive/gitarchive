package git

import (
	"bytes"
	"errors"
	"expvar"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Fetch fetches the git repo at gitURL and the returns the refs.
//
// It supports git:// and http(s):// URLs.
//
// Sideband messages from the git server are writetn to msgW, and the
// number of bytes fetched is incremented in bwCounter. bwCounter is
// incremented here to get fine-grained metrics.
func Fetch(gitURL string, haves map[string]struct{}, msgW io.Writer,
	bwCounter *expvar.Int) (refs map[string]string, r io.ReadCloser, err error) {

	u, err := url.Parse(gitURL)
	if err != nil {
		return nil, nil, err
	}

	switch u.Scheme {
	case "http", "https":
		refs, r, err = fetchHTTP(gitURL, haves)
	case "git":
		refs, r, err = fetchGIT(gitURL, haves)
	default:
		return nil, nil, errors.New("unsupported Scheme " + u.Scheme)
	}

	if err != nil {
		return nil, nil, err
	}

	if r == nil {
		// We came up with no wants. We already have all the objects.
		return refs, nil, nil
	}

	sbr := &sideBandReader{Upstream: r, MsgW: msgW}
	cr := &countingReader{Upstream: sbr, Counter: bwCounter}

	// Peek into the first 32 bytes to make sure it's not an empty
	// packfile.
	// Needs to be > 32 bytes (hdr is 12, trailer is 20).
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, cr, 64)
	if err != io.EOF && err != nil {
		r.Close()
		return nil, nil, err
	}
	if n == 32 {
		r.Close()
		return refs, nil, nil
	}
	rc := struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(&buf, cr),
		Closer: r,
	}
	return refs, rc, nil
}

func fetchGIT(gitURL string, haves map[string]struct{}) (refs map[string]string, r io.ReadCloser, err error) {
	u, _ := url.Parse(gitURL)
	port := "9418"
	host := u.Host
	if h, p, err := net.SplitHostPort(host); err == nil {
		port = p
		host = h
	}

	conn, err := net.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return
	}
	command := "git-upload-pack " + u.Path + "\x00host=" + host + "\x00"
	conn.Write([]byte(fmt.Sprintf("%04x%s", len(command)+4, command)))

	refs, err = ParseSmartResponse(conn, true)
	if err != nil {
		conn.Close()
		return
	}

	resp := buildResponse(refs, haves)
	if resp == nil {
		conn.Close()
		return
	}

	_, err = io.Copy(conn, resp)
	if err != nil {
		conn.Close()
		return
	}

	return refs, conn, nil
}

func buildResponse(refs map[string]string, haves map[string]struct{}) *bytes.Buffer {
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
		if strings.HasSuffix(name, "^{}") {
			continue
		}
		wants = append(wants, refs[name])
	}
	sort.Strings(wants)

	if len(wants) == 0 {
		return nil
	}

	resp := &bytes.Buffer{}
	last := ""
	for _, want := range wants {
		if last == want {
			continue
		}
		command := "want " + want
		if last == "" {
			command += " ofs-delta side-band-64k thin-pack"
			command += " agent=github.com/thecodearchive/gitarchive/git"
		}
		command += "\n"
		resp.WriteString(fmt.Sprintf("%04x%s", len(command)+4, command))
		last = want
	}
	resp.WriteString("0000")
	for have := range haves { // TODO: sort the haves
		command := "have " + have + "\n"
		resp.WriteString(fmt.Sprintf("%04x%s", len(command)+4, command))
	}
	resp.WriteString("0009done\n")

	return resp
}

func fetchHTTP(gitURL string, haves map[string]struct{}) (refs map[string]string, r io.ReadCloser, err error) {
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
	if resp.StatusCode == 401 || resp.StatusCode == 404 {
		return nil, nil, RemoteError{resp.Status}
	}
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("GET /info/refs: %d", resp.StatusCode)
	}
	refs, err = ParseSmartResponse(resp.Body, false)
	if err != nil {
		return
	}

	body := buildResponse(refs, haves)
	if body == nil {
		return
	}

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

	return refs, resp.Body, nil
}
