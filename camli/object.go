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

// PutObject uploads a blob to Camlistore.
func (u *Uploader) PutObject(r io.Reader) (string, error) {
	bb := schema.NewBuilder()
	bb.SetType("bytes")
	//bb.SetRawStringField("sha1", sha)
	ref, err := schema.WriteFileMap(u.c, bb, r)
	return ref.String(), err
}

// PutObject uploads a blob to Camlistore.
func (u *Uploader) GetObject(ref string) (*schema.FileReader, error) {
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
