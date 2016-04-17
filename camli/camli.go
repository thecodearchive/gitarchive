// Package camli provides a wrapper around the Camlistore client for
// storing git blobs.
package camli

import (
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
