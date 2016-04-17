package camli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/search"
)

// Repo is our Camlistore scheme to model the state of a
// particular repo at a particular point in time.
type Repo struct {
	Name      string
	Parent    string
	Retrieved time.Time
	Refs      map[string]string
	Packfiles []string
}

// PutRepo stores a Repo in Camlistore.
func (u *Uploader) PutRepo(r *Repo) error {
	bb := schema.NewBuilder()
	bb.SetType("git-repo")
	bb.SetRawStringField("parent", r.Parent)
	bb.SetRawStringField("retrieved", schema.RFC3339FromTime(r.Retrieved))
	if refs, err := schema.NewJSONObject(r.Refs); err == nil {
		bb.SetRawField("refs", refs)
	} else {
		return err
	}
	if packfiles, err := schema.NewJSONObject(r.Packfiles); err == nil {
		bb.SetRawField("packfiles", packfiles)
	} else {
		return err
	}

	j := bb.Blob().JSON()
	reporef := blob.SHA1FromString(j)
	_, err := uploadString(u.c, reporef, j)

	log.Printf("stored repo: %s on %s", r.Name, reporef)

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
	contentattr := schema.NewSetAttributeClaim(pn, "camliContent", reporef.String())
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
