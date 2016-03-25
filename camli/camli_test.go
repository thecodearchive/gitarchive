package camli

import (
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"camlistore.org/pkg/blobserver/dir"
	"camlistore.org/pkg/client"

	"gopkg.in/src-d/go-git.v3/core"
)

// These tests require a local running instance of Camlistore.
// `devam server -wipe` should work.

// object implements core.Object for testing purposes.
// TODO maybe move away from this go-git package...
type object struct {
	hash     [20]byte
	contents []byte
	objtype  core.ObjectType
}

func (o *object) Hash() core.Hash         { return o.hash }
func (o *object) Type() core.ObjectType   { return o.objtype }
func (o *object) Size() int64             { return int64(len(o.contents)) }
func (o *object) Reader() io.Reader       { return bytes.NewBuffer(o.contents) }
func (o *object) SetType(core.ObjectType) {}             // Nope.
func (o *object) SetSize(int64)           {}             // Nope.
func (o *object) Writer() io.Writer       { return nil } // Nope.

func strtohash(s string) [20]byte {
	bs, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	var b [20]byte
	copy(b[:], bs)
	return b
}

func mustMkdir(t *testing.T, fn string, mode int) {
	if err := os.Mkdir(fn, 0700); err != nil {
		t.Errorf("error creating dir %s: %v", fn, err)
	}
}

func testWithTempDir(t *testing.T, fn func(tempDir string)) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("error creating temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	confDir := filepath.Join(tempDir, "conf")
	mustMkdir(t, confDir, 0700)
	defer os.Setenv("CAMLI_CONFIG_DIR", os.Getenv("CAMLI_CONFIG_DIR"))
	os.Setenv("CAMLI_CONFIG_DIR", confDir)
	if err := ioutil.WriteFile(filepath.Join(confDir, "client-config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	fn(tempDir)
}

func TestPutBlobs(t *testing.T) {
	objects := []*object{
		{
			contents: []byte("hello"),
			objtype:  core.TreeObject,
			hash:     strtohash("cbb918f93e0b6cdc9632f3ce0f94805cd7c3b498"),
		},
		{
			contents: []byte("just"),
			objtype:  core.CommitObject,
			hash:     strtohash("7343b97e4d39e53a6befe9556a7ee65534569138"),
		},
		{
			contents: []byte("testing\nthis"),
			objtype:  core.BlobObject,
			hash:     strtohash("9bedb58b7cf0116f4e7994971b291ddb7cfb2539"),
		},
	}

	testWithTempDir(t, func(tempDir string) {
		blobDestDir := filepath.Join(tempDir, "blob_dest")
		mustMkdir(t, blobDestDir, 0700)

		ss, err := dir.New(blobDestDir)
		if err != nil {
			t.Fatalf("Couldn't create dir for blob storage: %v", err)
		}
		// Use the local storage backend.
		uploader := &Uploader{
			c: client.NewStorageClient(ss),
		}

		for _, o := range objects {
			err := uploader.PutObject(o)
			if err != nil {
				t.Errorf("Couldn't put an object: %v", err)
			}
		}

		// TODO(s) also test permanode creation and retrieval
	})
}
