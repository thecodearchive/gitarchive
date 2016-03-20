package git

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/src-d/go-git.v3/core"
	"gopkg.in/src-d/go-git.v3/formats/packfile"
	"gopkg.in/src-d/go-git.v3/storage/memory"
)

var GitParseError = errors.New("failed parsing the git protocol")

func ParseSmartResponse(body io.Reader) (refs map[string]string, caps []string, err error) {
	// https://github.com/git/git/blob/master/Documentation/technical/http-protocol.txt
	refs = make(map[string]string)
	state := "service-header"
	for {
		pktLenHex := make([]byte, 4)
		if _, err := io.ReadFull(body, pktLenHex); err == io.EOF {
			return refs, caps, nil
		} else if err != nil {
			return nil, nil, err
		}
		pktLen, err := strconv.ParseUint(string(pktLenHex), 16, 16)
		if err != nil {
			return nil, nil, err
		}

		// "0000" marker
		if pktLen == 0 {
			continue
		}

		lineBuf := make([]byte, pktLen-4)
		if _, err := io.ReadFull(body, lineBuf); err != nil {
			return nil, nil, err
		}
		line := string(lineBuf)
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}

		switch state {
		case "service-header":
			if line != "# service=git-upload-pack" {
				return nil, nil, GitParseError
			}
			state = "head"

		case "head":
			parts := strings.SplitN(line, "\x00", 2)
			if len(parts) != 2 {
				return nil, nil, GitParseError
			}

			refParts := strings.SplitN(parts[0], " ", 2)
			if len(refParts) != 2 {
				return nil, nil, GitParseError
			}
			refs[refParts[1]] = refParts[0]

			caps = strings.Split(parts[1], " ")

			state = "ref-list"

		case "ref-list":
			refParts := strings.SplitN(line, " ", 2)
			if len(refParts) != 2 {
				return nil, nil, GitParseError
			}
			refs[refParts[1]] = refParts[0]

		default:
			panic("unexpected state")
		}
	}
}

func ParseUploadPackResponse(body io.Reader, msgW io.Writer) (objs map[core.Hash]core.Object, err error) {
	r := &sideBandReader{Upstream: body, MsgW: msgW}
	st := memory.NewObjectStorage()
	_, err = packfile.NewReader(r).Read(st)
	if r.Errors != nil {
		err = fmt.Errorf("remote error: %s", r.Errors)
	}
	return st.Objects, err
}

type sideBandReader struct {
	Upstream io.Reader
	buffer   []byte

	MsgW   io.Writer
	Errors []byte
}

func (s *sideBandReader) Read(p []byte) (n int, err error) {
	// Did I ever mention I love byte slices and the io.Reader interface?

	if len(s.buffer) > 0 {
		n := copy(p, s.buffer)
		// I wonder if this release the memory when len(buffer) becomes 0...
		s.buffer = s.buffer[n:]
		return n, nil
	}

	for {
		pktLenHex := make([]byte, 4)
		if _, err := io.ReadFull(s.Upstream, pktLenHex); err != nil {
			return 0, err
		}
		pktLen, err := strconv.ParseUint(string(pktLenHex), 16, 16)
		if err != nil {
			return 0, err
		}

		// "0000" marker
		if pktLen == 0 {
			continue
		}

		pkt := make([]byte, pktLen-4)
		if _, err := io.ReadFull(s.Upstream, pkt); err != nil {
			return 0, err
		}

		if len(pkt) == 4+len("NAK\n") && string(pkt) == "NAK\n" {
			continue
		}

		switch pkt[0] {
		case 1:
			n := copy(p, pkt[1:])
			s.buffer = pkt[1+n:]
			return n, nil
		case 2:
			s.MsgW.Write(pkt[1:]) // ignoring the error, it's just messages
		case 3:
			s.Errors = append(s.Errors, pkt[1:]...)
		}
	}
}
