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

package client

import (
	"bytes"
	"fmt"
	"http"
	"io"
	"json"
	"os"
	"strings"

	"camli/blobref"
	"url"
)

type removeResponse struct {
	Removed []string `json:"removed"`
}

// Remove the list of blobs. An error is returned if the server failed to
// remove a blob. Removing a non-existent blob isn't an error.
func (c *Client) RemoveBlobs(blobs []*blobref.BlobRef) os.Error {
	url_ := fmt.Sprintf("%s/camli/remove", c.server)
	params := make(url.Values)           // "blobN" -> BlobRefStr
	needsDelete := make(map[string]bool) // BlobRefStr -> true
	for n, b := range blobs {
		if b == nil {
			return os.NewError("Cannot delete nil blobref")
		}
		key := fmt.Sprintf("blob%v", n+1)
		params.Add(key, b.String())
		needsDelete[b.String()] = true
	}

	req, err := http.NewRequest("POST", url_, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("Error creating RemoveBlobs POST request: %v", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	c.addAuthHeader(req)
	resp, err := c.httpClient.Do(req)

	if err != nil {
		return os.NewError(fmt.Sprintf("Got status code %d from blobserver for remove %s", resp.StatusCode, params.Encode()))
	}

	// The only valid HTTP responses are 200.
	if resp.StatusCode != 200 {
		return os.NewError(fmt.Sprintf("Invalid http response %d in remove response", resp.StatusCode))
	}

	// TODO: LimitReader here for paranoia
	buf := new(bytes.Buffer)
	io.Copy(buf, resp.Body)
	resp.Body.Close()
	var remResp removeResponse
	if jerr := json.Unmarshal(buf.Bytes(), &remResp); jerr != nil {
		return os.NewError(fmt.Sprintf("Failed to parse remove response %q: %s", buf.String(), jerr))
	}
	for _, value := range remResp.Removed {
		needsDelete[value] = false, false
	}

	if len(needsDelete) > 0 {
		return os.NewError(fmt.Sprintf("Failed to remove blobs %s", strings.Join(stringKeys(needsDelete), ", ")))
	}

	return nil
}

// Remove the single blob. An error is returned if the server failed to remove
// the blob. Removing a non-existent blob isn't an error.
func (c *Client) RemoveBlob(b *blobref.BlobRef) os.Error {
	return c.RemoveBlobs([]*blobref.BlobRef{b})
}

func stringKeys(m map[string]bool) (s []string) {
	s = make([]string, 0, len(m))
	for key, _ := range m {
		s = append(s, key)
	}
	return
}
