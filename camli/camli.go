// Package camli provides a wrapper around the Camlistore client for
// storing git blobs.
package camli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/cmdmain"
	"camlistore.org/pkg/httputil"
	"camlistore.org/pkg/osutil"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/search"

	"gopkg.in/src-d/go-git.v3/core"
)

var Verbose = false

func init() {
	osutil.AddSecretRingFlag()
	flag.Parse()
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
			&client.TransportConfig{
				Verbose: Verbose,
			}))
	stats := c.HTTPStats()

	if Verbose {
		c.SetLogger(log.New(cmdmain.Stderr, "", log.LstdFlags))
	} else {
		c.SetLogger(nil)
	}

	return &Uploader{
		c:     c,
		stats: stats,
	}
}

// PutObject uploads a blob to Camlistore.
func (u *Uploader) PutObject(obj core.Object) error {
	sum := [20]byte(obj.Hash())
	head := obj.Type().Bytes()
	head = append(head, ' ')
	head = strconv.AppendInt(head, obj.Size(), 10)
	head = append(head, 0)
	r := io.MultiReader(bytes.NewReader(head), obj.Reader())

	result, err := u.c.Upload(&client.UploadHandle{
		BlobRef:  blob.MustParse(fmt.Sprintf("sha1-%x", sum)),
		Size:     uint32(obj.Size()) + uint32(len(head)),
		Contents: r,
	})
	if err != nil {
		log.Printf("couldn't store object: %x", sum)
		return err
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
	// TODO switch to Time3339 so we can to query this.
	Retrieved time.Time
	Refs      map[string]string
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
		return blob.Ref{}, nil, errors.New("repo not found")
	}
	return res.Blobs[0].Blob, res.Describe.Meta, nil
}

// GetRepo querys for a repo permanode with name, and returns its
// Repo object.
func (u *Uploader) GetRepo(name string) (*Repo, error) {
	pn, meta, err := u.findRepo(name)
	if err != nil {
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

// TODO:
//   - query permanodes for title==name to check if the repo exists
//     - if yes, upload and set contentattr
//     - if no, upload and create a new permanode, set title & contentattr
//   - root nodes?
