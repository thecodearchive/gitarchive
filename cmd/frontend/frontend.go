package main

import (
	"expvar"

	"github.com/thecodearchive/gitarchive/index"
	"google.golang.org/cloud/storage"
)

type Frontend struct {
	i      *index.Index
	bucket *storage.BucketHandle

	exp *expvar.Map
}

func (f *Frontend) Run() error {
	return nil
}

func (f *Frontend) Stop() {}
