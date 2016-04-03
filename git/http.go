package git

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/src-d/go-git.v3/core"
)

func Fetch(gitURL string, haves map[string]struct{}, uploader core.ObjectStorage,
	msgW io.Writer) (refs map[string]string, caps []string, err error) {

	req, err := http.NewRequest("GET", gitURL+"/info/refs?service=git-upload-pack", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "github.com/thecodearchive/gitarchive/git")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("GET /info/refs: %d", resp.StatusCode)
	}
	refs, caps, err = ParseSmartResponse(resp.Body)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	req.Header.Set("User-Agent", "github.com/thecodearchive/gitarchive/git")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("POST /git-upload-pack: %d", resp.StatusCode)
	}

	err = ParseUploadPackResponse(resp.Body, uploader, msgW)

	return
}
