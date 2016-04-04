package camli

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"gopkg.in/src-d/go-git.v3/core"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/schema"
)

func gitBlobReader(obj core.Object) (io.Reader, uint32) {
	head := obj.Type().Bytes()
	head = append(head, ' ')
	head = strconv.AppendInt(head, obj.Size(), 10)
	head = append(head, 0)
	r := io.MultiReader(bytes.NewReader(head), obj.Reader())
	size := uint32(obj.Size()) + uint32(len(head))
	return r, size
}

// PutObject uploads a blob to Camlistore.
func (u *Uploader) PutObject(obj core.Object) (bool, error) {
	sum := [20]byte(obj.Hash())
	r, size := gitBlobReader(obj)
	result, err := u.c.Upload(&client.UploadHandle{
		BlobRef:  blob.MustParse(fmt.Sprintf("sha1-%x", sum)),
		Size:     size,
		Contents: r,
	})
	ref, err := u.putLargeObject(fmt.Sprintf("%x", sum), r)
	if err != nil {
		return false, fmt.Errorf("couldn't store %x: %v", sum, err)
	}
	log.Printf("stored object: %x on %s", sum, ref)
	return result.Skipped, nil
}

// The largeBlob code below is loosely based on the file schema
// from Camlistore.

const (
	// maxBlobSize is the largest blob we make when cutting up
	// a file. It's also the threshold on which we switch to using
	// the largeBlob schema instead of storing the files directly.
	maxBlobSize = 1 << 10
)

func (u *Uploader) putLargeObject(sha string, r io.Reader) (blob.Ref, error) {
	bb := schema.NewBuilder()
	bb.SetType("git-object")
	bb.SetRawStringField("sha1", sha)
	err := schema.WriteFileChunks(u.c, bb, r)
	if err != nil {
		return blob.Ref{}, err
	}
	j := bb.Blob().JSON()
	br := blob.SHA1FromString(j)
	log.Printf("wrote large blob %s", br)
	return uploadString(u.c, br, j)
}

func uploadString(bs blobserver.StatReceiver, br blob.Ref, s string) (blob.Ref, error) {
	if !br.Valid() {
		panic("invalid blobref")
	}
	hasIt, err := serverHasBlob(bs, br)
	if err != nil {
		return blob.Ref{}, err
	}
	if hasIt {
		return br, nil
	}
	_, err = blobserver.ReceiveNoHash(bs, br, strings.NewReader(s))
	if err != nil {
		return blob.Ref{}, err
	}
	return br, nil
}

func serverHasBlob(bs blobserver.BlobStatter, br blob.Ref) (have bool, err error) {
	_, err = blobserver.StatBlob(bs, br)
	if err == nil {
		have = true
	} else if err == os.ErrNotExist {
		err = nil
	}
	return
}
