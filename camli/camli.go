// Package camli provides a wrapper around the Camlistore client for
// storing git blobs.
package camli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/httputil"
	"gopkg.in/src-d/go-git.v3/core"
	"gopkg.in/src-d/go-git.v3/storage/memory"
)

func AddFlags() {
	client.AddFlags()
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

func (u *Uploader) New() (core.Object, error) {
	// Lazy, just used the core in memory objects for now.
	return &memory.Object{}, nil
}

func (u *Uploader) Set(obj core.Object) (core.Hash, error) {
	_, err := u.PutObject(obj)
	return obj.Hash(), err
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
