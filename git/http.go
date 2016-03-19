package git

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/src-d/go-git.v3/core"
)

func Clone(gitURL string) (objs map[core.Hash]core.Object, refs map[string]string, caps []string, err error) {
	resp, err := http.Get(gitURL + "/info/refs?service=git-upload-pack")
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, nil, fmt.Errorf("GET /info/refs: %d", resp.StatusCode)
	}
	refs, caps, err = ParseSmartResponse(resp.Body)
	if err != nil {
		return nil, nil, nil, err
	}

	wants := []string{refs["HEAD"]}
	for ref := range refs {
		if strings.HasPrefix(ref, "refs/heads/") {
			wants = append(wants, refs[ref])
		}
	}
	sort.Strings(wants)

	body := &bytes.Buffer{}
	last := ""
	for _, want := range wants {
		if last == want {
			continue
		}
		command := "want " + want
		if last == "" {
			command += " ofs-delta"
		}
		command += "\n"
		body.WriteString(fmt.Sprintf("%04x%s", len(command)+4, command))
		last = want
	}
	body.WriteString("00000009done\n")

	req, err := http.NewRequest("POST", gitURL+"/git-upload-pack", body)
	if err != nil {
		return nil, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, nil, fmt.Errorf("POST /git-upload-pack: %d", resp.StatusCode)
	}

	objs, err = ParseUploadPackResponse(resp.Body)
	return
}
