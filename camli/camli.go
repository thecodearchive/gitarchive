// Package camli provides a wrapper around the Camlistore client for
// storing git blobs.
package camli

import (
	"os"

	"camlistore.org/pkg/auth"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/httputil"
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
func NewUploader() (*Uploader, error) {
	a, err := auth.FromEnv()
	if err == auth.ErrNoAuth {
		a = auth.None{}
	} else if err != nil {
		return nil, err
	}

	c := client.NewFromParams(os.Getenv("CAMLI_SERVER"), a,
		client.OptionTransportConfig(&client.TransportConfig{}))
	stats := c.HTTPStats()

	u := &Uploader{
		c:     c,
		stats: stats,
	}
	_, err = u.GetRepo("https://github.com/thecodearchive/gitarchive.git")
	return u, err
}
