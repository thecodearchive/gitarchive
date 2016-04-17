package camli

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"camlistore.org/pkg/blobserver/dir"
	"camlistore.org/pkg/client"
)

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
	objects := []string{
		"hello",
		"hi", "test\ning",
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

		refs := make([]string, len(objects))
		for i := range objects {
			refs[i], err = uploader.PutObject(bytes.NewBufferString(objects[i]))
			if err != nil {
				t.Errorf("Couldn't put an object: %v", err)
			}
		}

		for i := range objects {
			r, err := uploader.GetObject(refs[i])
			if err != nil {
				t.Errorf("Couldn't get an object: %v", err)
			}
			contents, err := ioutil.ReadAll(r)
			if err != nil {
				t.Errorf("Couldn't read object: %v", err)
			}
			if string(contents) != objects[i] {
				t.Errorf("Got back unexpected contents. got: %s want: %s", string(contents), objects[i])
			}
		}

		// TODO(s) also test permanode creation and retrieval
	})
}
