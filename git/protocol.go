package git

import (
	"expvar"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type GitParseError struct {
	state string
}

func (e GitParseError) Error() string {
	return "failed parsing the git protocol at state: " + e.state
}

type RemoteError struct {
	Message string
}

func (e RemoteError) Error() string {
	return "remote error: " + e.Message
}

func ParseSmartResponse(body io.Reader, gitProto bool) (refs map[string]string, err error) {
	// https://github.com/git/git/blob/master/Documentation/technical/http-protocol.txt
	refs = make(map[string]string)
	state := "service-header"
	if gitProto {
		state = "head"
	}
	for {
		pktLenHex := make([]byte, 4)
		if _, err := io.ReadFull(body, pktLenHex); err == io.EOF {
			return refs, nil
		} else if err != nil {
			return nil, err
		}
		pktLen, err := strconv.ParseUint(string(pktLenHex), 16, 16)
		if err != nil {
			return nil, err
		}

		// "0000" marker
		if pktLen == 0 {
			if gitProto {
				return refs, nil
			} else {
				continue
			}
		}

		lineBuf := make([]byte, pktLen-4)
		if _, err := io.ReadFull(body, lineBuf); err != nil {
			return nil, err
		}
		line := string(lineBuf)
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}

		switch state {
		case "service-header":
			if line != "# service=git-upload-pack" {
				return nil, GitParseError{state}
			}
			state = "head"

		case "head":
			if strings.HasPrefix(line, "ERR") {
				return nil, RemoteError{strings.Trim(line[len("ERR"):], " \n")}
			}

			parts := strings.SplitN(line, "\x00", 2)
			if len(parts) != 2 {
				return nil, GitParseError{state}
			}

			refParts := strings.SplitN(parts[0], " ", 2)
			if len(refParts) != 2 {
				return nil, GitParseError{state}
			}
			refs[refParts[1]] = refParts[0]

			// caps = strings.Split(parts[1], " ")

			state = "ref-list"

		case "ref-list":
			refParts := strings.SplitN(line, " ", 2)
			if len(refParts) != 2 {
				return nil, GitParseError{state}
			}
			refs[refParts[1]] = refParts[0]

		default:
			panic("unexpected state")
		}
	}
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
			return 0, fmt.Errorf("remote error: %s", pkt[1:])
		}
	}
}

type countingReader struct {
	Upstream  io.Reader
	BytesRead int64
	Counter   *expvar.Int
}

func (r *countingReader) Read(p []byte) (n int, err error) {
	n, err = r.Upstream.Read(p)
	r.BytesRead += int64(n)
	if r.Counter != nil {
		r.Counter.Add(int64(n))
	}
	return
}
