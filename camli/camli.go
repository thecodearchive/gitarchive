// Package camli provides a wrapper around the Camlistore client for
// storing git blobs.
package camli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/httputil"
	"camlistore.org/pkg/osutil"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/search"
	"go4.org/types"
	"gopkg.in/src-d/go-git.v3/core"
	"gopkg.in/src-d/go-git.v3/storage/memory"
)

func RegisterFlags() {
	osutil.AddSecretRingFlag()
}

type Uploader struct {
	c     *client.Client
	stats *httputil.StatsTransport
	// TODO fdgate, localcache.
}

// NewUploader returns a git blob uploader.
func NewUploader() *Uploader {
	c := client.NewOrFail(
		client.OptionTransportConfig(
			&client.TransportConfig{}))
	stats := c.HTTPStats()

	return &Uploader{
		c:     c,
		stats: stats,
	}
}

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
func (u *Uploader) PutObject(obj core.Object) error {
	sum := [20]byte(obj.Hash())
	r, size := gitBlobReader(obj)
	result, err := u.c.Upload(&client.UploadHandle{
		BlobRef:  blob.MustParse(fmt.Sprintf("sha1-%x", sum)),
		Size:     size,
		Contents: r,
	})
	if err != nil {
		return fmt.Errorf("couldn't store %x: %v", sum, err)
	}
	if result.Skipped {
		log.Printf("object %x already on the server", sum)
	} else {
		log.Printf("stored object: %x", sum)
	}
	return nil
}

// Repo represets is our Camlistore scheme to model the state of a
// particular repo at a particular point in time.
type Repo struct {
	CamliVersion int
	CamliType    string
	Name         string
	Retrieved    types.Time3339
	Refs         map[string]string
}

// PutRepo stores a Repo in Camlistore.
func (u *Uploader) PutRepo(r *Repo) error {
	// Set the camli specific fields.
	r.CamliVersion = 1
	r.CamliType = "camliGitRepo"

	// Upload the repo object.
	j, err := json.Marshal(r)
	if err != nil {
		return err
	}
	h := blob.NewHash()
	size, err := io.Copy(h, bytes.NewReader(j))
	if err != nil {
		return err
	}
	reporef, err := u.c.Upload(&client.UploadHandle{
		BlobRef:  blob.RefFromHash(h),
		Size:     uint32(size),
		Contents: bytes.NewReader(j),
	})
	if err != nil {
		return err
	}
	log.Printf("stored repo: %s on %s", r.Name, reporef.BlobRef)

	// Update or create its permanode.
	pn, _, err := u.findRepo(r.Name)
	if err != nil {
		// Create a new one.
		res, err := u.c.UploadNewPermanode()
		if err != nil {
			return err
		}
		pn = res.BlobRef
		log.Printf("created permanode: %s", pn)

		titleattr := schema.NewSetAttributeClaim(pn, "title", r.Name)
		claimTime := time.Now()
		titleattr.SetClaimDate(claimTime)
		signer, err := u.c.Signer()
		if err != nil {
			return err
		}
		signed, err := titleattr.SignAt(signer, claimTime)
		if err != nil {
			return fmt.Errorf("couldn't to sign title claim")
		}
		_, err = u.c.Upload(client.NewUploadHandleFromString(signed))
	}
	contentattr := schema.NewSetAttributeClaim(pn, "camliContent", reporef.BlobRef.String())
	claimTime := time.Now()
	contentattr.SetClaimDate(claimTime)
	signer, err := u.c.Signer()
	if err != nil {
		return err
	}
	signed, err := contentattr.SignAt(signer, claimTime)
	if err != nil {
		return fmt.Errorf("couldn't to sign content claim")
	}
	_, err = u.c.Upload(client.NewUploadHandleFromString(signed))
	return err
}

var repoNotFoundErr = errors.New("repo not found")

func (u *Uploader) findRepo(name string) (blob.Ref, search.MetaMap, error) {
	res, err := u.c.Query(&search.SearchQuery{
		Limit: 1,
		Constraint: &search.Constraint{
			Permanode: &search.PermanodeConstraint{
				Attr: "title", Value: name,
			},
		},
		Describe: &search.DescribeRequest{},
	})
	if err != nil {
		return blob.Ref{}, nil, err
	}
	if len(res.Blobs) < 1 {
		return blob.Ref{}, nil, repoNotFoundErr
	}
	return res.Blobs[0].Blob, res.Describe.Meta, nil
}

// GetRepo querys for a repo permanode with name, and returns its
// Repo object.
func (u *Uploader) GetRepo(name string) (*Repo, error) {
	pn, meta, err := u.findRepo(name)
	if err == repoNotFoundErr {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	ref, ok := meta[pn.String()].ContentRef()
	if !ok {
		return nil, errors.New("couldn't find repo data (but there's a permanode)")
	}
	r, _, err := u.c.Fetch(ref)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var repo Repo
	err = json.Unmarshal(body, &repo)
	return &repo, err
}

func (u *Uploader) New() (core.Object, error) {
	// Lazy, just used the core in memory objects for now.
	return &memory.Object{}, nil
}

func (u *Uploader) Set(obj core.Object) (core.Hash, error) {
	return obj.Hash(), u.PutObject(obj)
}

func (u *Uploader) Get(hash core.Hash) (core.Object, error) {
	r, _, err := u.c.Fetch(blob.MustParse(fmt.Sprintf("sha1-%s", hash)))
	if err != nil {
		return nil, err
	}

	obj := &memory.Object{}
	buf := bufio.NewReader(r)
	tstr, err := buf.ReadBytes(' ')
	if err != nil {
		return nil, err
	}
	t, err := core.ParseObjectType(string(tstr[:len(tstr)-1]))
	if err != nil {
		return nil, err
	}
	obj.SetType(t)

	sizestr, err := buf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	size, err := strconv.Atoi(string(sizestr[:len(sizestr)-1]))
	if err != nil {
		return nil, err
	}
	obj.SetSize(int64(size))

	_, err = io.Copy(obj.Writer(), buf)
	return obj, err
}

func (u *Uploader) Iter(core.ObjectType) core.ObjectIter {
	panic("Uploader.Iter called")
}
