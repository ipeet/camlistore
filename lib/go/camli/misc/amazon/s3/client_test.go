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

package s3

import (
	"http"
	"os"
	"testing"
)

var tc *Client

func getTestClient(t *testing.T) bool {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secret := os.Getenv("AWS_ACCESS_KEY_SECRET")
	if accessKey == "" || secret == "" {
		t.Logf("Skipping test; no AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY_SECRET set in environment")
		return false
	}
	tc = &Client{&Auth{accessKey, secret}, http.DefaultClient}
	return true
}

func TestBuckets(t *testing.T) {
	if !getTestClient(t) {
		return
	}
	tc.Buckets()
}
