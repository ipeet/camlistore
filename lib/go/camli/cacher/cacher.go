/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cacher

import (
	"io"
	"os"

	"camli/blobref"
	"camli/blobserver"
)

func NewCachingFetcher(cacheTarget blobserver.Cache, sfetcher blobref.StreamingFetcher) blobref.SeekFetcher {
	return &CachingFetcher{cacheTarget, sfetcher}
}

type CachingFetcher struct {
	c  blobserver.Cache
	sf blobref.StreamingFetcher
}

var _ blobref.StreamingFetcher = (*CachingFetcher)(nil)
var _ blobref.SeekFetcher = (*CachingFetcher)(nil)

func (cf *CachingFetcher) FetchStreaming(br *blobref.BlobRef) (file io.ReadCloser, size int64, err os.Error) {
	file, size, err = cf.c.Fetch(br)
	if err == nil {
		return
	}
	cf.faultIn(br)
	return cf.c.Fetch(br)
}

func (cf *CachingFetcher) Fetch(br *blobref.BlobRef) (file blobref.ReadSeekCloser, size int64, err os.Error) {
	file, size, err = cf.c.Fetch(br)
	if err == nil {
		return
	}
	cf.faultIn(br)
	return cf.c.Fetch(br)
}

func (cf *CachingFetcher) faultIn(br *blobref.BlobRef) os.Error {
	sblob, _, err := cf.sf.FetchStreaming(br)
	if err != nil {
		return err
	}

	_, err = cf.c.ReceiveBlob(br, sblob)
	return err
}
