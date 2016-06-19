package git

import (
	"bytes"
	"reflect"
	"testing"
)

var smartResponse = []byte(`001e# service=git-upload-pack
0000010721d7ee08fb632ae032079e10b41f5987531ba0cc HEAD` + "\x00" + `multi_ack thin-pack side-band side-band-64k ofs-delta shallow no-progress include-tag multi_ack_detailed no-done symref=HEAD:refs/heads/master agent=git/2:2.6.5~simonsj-receive-refUpdateCommandLimit-1387-g4aa12b5
00418f07421ada5140010afd7b00b313781401cd36b5 refs/heads/gh-pages
003f21d7ee08fb632ae032079e10b41f5987531ba0cc refs/heads/master
003e7661c0ea4e01cfed9213bee6e5e95370466d3f00 refs/pull/1/head
003f8dc6b0520ec519c16b59dcc53f894c34dc4c5b89 refs/pull/1/merge
003f2c703ebabaff3a198f704a3355152f6caf43a3c9 refs/pull/10/head
003f991e7b86c792ff58ee65217c76cf3fe4ccfb6d5c refs/pull/11/head
0040d6b92fed1e0f7a43f7de49a2b8acf2fce7c1353b refs/pull/11/merge
003f3ecca813a5d0c6d6e5a074cdb70675d849cd1fe9 refs/pull/14/head
003fe7f63d066ff67d52a32e5d24b24b1ea26547c5bb refs/pull/17/head
003fc4178c9c682caa22a54692469fb354b0fc3f5f42 refs/pull/18/head
00406467d8a2f03ad50f24ba2bc786378f2d2aa6b204 refs/pull/18/merge
003ff04646d08d6f46f37b02fb1a7dbc0872a5e71c7f refs/pull/19/head
003eb24086faed246eeddf77986d7dbc9750fef82645 refs/pull/8/head
0000`)

var smartResponseRefs = map[string]string{"HEAD": "21d7ee08fb632ae032079e10b41f5987531ba0cc", "refs/heads/gh-pages": "8f07421ada5140010afd7b00b313781401cd36b5", "refs/heads/master": "21d7ee08fb632ae032079e10b41f5987531ba0cc", "refs/pull/1/head": "7661c0ea4e01cfed9213bee6e5e95370466d3f00", "refs/pull/1/merge": "8dc6b0520ec519c16b59dcc53f894c34dc4c5b89", "refs/pull/10/head": "2c703ebabaff3a198f704a3355152f6caf43a3c9", "refs/pull/11/head": "991e7b86c792ff58ee65217c76cf3fe4ccfb6d5c", "refs/pull/11/merge": "d6b92fed1e0f7a43f7de49a2b8acf2fce7c1353b", "refs/pull/14/head": "3ecca813a5d0c6d6e5a074cdb70675d849cd1fe9", "refs/pull/17/head": "e7f63d066ff67d52a32e5d24b24b1ea26547c5bb", "refs/pull/18/head": "c4178c9c682caa22a54692469fb354b0fc3f5f42", "refs/pull/18/merge": "6467d8a2f03ad50f24ba2bc786378f2d2aa6b204", "refs/pull/19/head": "f04646d08d6f46f37b02fb1a7dbc0872a5e71c7f", "refs/pull/8/head": "b24086faed246eeddf77986d7dbc9750fef82645"}

// var smartResponseCaps = []string{"multi_ack", "thin-pack", "side-band", "side-band-64k", "ofs-delta", "shallow", "no-progress", "include-tag", "multi_ack_detailed", "no-done", "symref=HEAD:refs/heads/master", "agent=git/2:2.6.5~simonsj-receive-refUpdateCommandLimit-1387-g4aa12b5"}

func TestParseSmartResponse(t *testing.T) {
	refs, err := ParseSmartResponse(bytes.NewReader(smartResponse), false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(refs, smartResponseRefs) {
		t.Fatalf("Wrong refs: %v", refs)
	}
	// if !reflect.DeepEqual(caps, smartResponseCaps) {
	// 	t.Fatalf("Wrong caps: %v", caps)
	// }
}
