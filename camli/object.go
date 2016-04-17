package camli

import (
	"errors"
	"io"
	"os"
	"strings"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/schema"
)

// PutObject uploads a blob, using the "bytes" schema. This will split the
// blob into smaller ones to help with deduplication and obey Camlistore
// blob size limit.
func (u *Uploader) PutObject(r io.Reader) (ref string, err error) {
	bb := schema.NewBuilder()
	bb.SetType("bytes")
	//bb.SetRawStringField("sha1", sha)
	br, err := schema.WriteFileMap(u.c, bb, r)
	return br.String(), err
}

// GetObject returns a reader for the "bytes" blob referenced by ref.
func (u *Uploader) GetObject(ref string) (r *schema.FileReader, err error) {
	br, ok := blob.Parse(ref)
	if ok {
		return schema.NewFileReader(u.c, br)
	}
	return nil, errors.New("invalid blob ref")
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
