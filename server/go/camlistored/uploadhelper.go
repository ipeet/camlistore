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

package main

import (
	"http"
	"os"

	"camli/httputil"
	"camli/schema"
)

func (ui *UIHandler) serveUploadHelper(rw http.ResponseWriter, req *http.Request) {
	rollSum := req.URL.Query().Get("rollsum") == "1"

	ret := make(map[string]interface{})
	defer httputil.ReturnJson(rw, ret)

	if ui.Storage == nil {
		ret["error"] = "No BlobRoot configured"
		ret["errorType"] = "server"
		return
	}

	mr, err := req.MultipartReader()
	if err != nil {
		ret["error"] = "reading body: " + err.String()
		ret["errorType"] = "server"
		return
	}

	got := make([]map[string]interface{}, 0)
	for {
		part, err := mr.NextPart()
		if err == os.EOF {
			break
		}
		if err != nil {
			ret["error"] = "reading body: " + err.String()
			ret["errorType"] = "server"
			break
		}
		fileName := part.FileName()
		if fileName == "" {
			continue
		}
		writeFn := schema.WriteFileFromReader
		if rollSum {
			writeFn = schema.WriteFileFromReaderRolling
		}
		br, err := writeFn(ui.Storage, fileName, part)

		if err == nil {
			got = append(got, map[string]interface{}{
				"filename": part.FileName(),
				"formname": part.FormName(),
				"fileref":  br.String(),
			})
		} else {
			ret["error"] = "writing to blobserver: " + err.String()
			return
		}
	}
	ret["got"] = got
}
