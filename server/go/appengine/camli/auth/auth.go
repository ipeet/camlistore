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

package auth

import (
	"encoding/base64"
	"fmt"
	"http"
	"os"
	"regexp"
	"strings"
)

var kBasicAuthPattern *regexp.Regexp = regexp.MustCompile(`^Basic ([a-zA-Z0-9\+/=]+)`)

var AccessPassword string

func TriedAuthorization(req *http.Request) bool {
	// Currently a simple test just using HTTP basic auth
	// (presumably over https); may expand.
	return req.Header.Get("Authorization") != ""
}

func SendUnauthorized(conn http.ResponseWriter) {
	realm := "camlistored"
	if pw := os.Getenv("CAMLI_ADVERTISED_PASSWORD"); pw != "" {
		realm = "Any username, password is: " + pw
	}
	conn.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", realm))
	conn.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(conn, "<h1>Unauthorized</h1>")
}

func IsAuthorized(req *http.Request) bool {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		return false
	}
	matches := kBasicAuthPattern.FindStringSubmatch(auth)
	if len(matches) != 2 {
		return false
	}
	encoded := matches[1]
	enc := base64.StdEncoding
	decBuf := make([]byte, enc.DecodedLen(len(encoded)))
	n, err := enc.Decode(decBuf, []byte(encoded))
	if err != nil {
		return false
	}
	userpass := strings.SplitN(string(decBuf[0:n]), ":", 2)
	if len(userpass) != 2 {
		fmt.Println("didn't get two pieces")
		return false
	}
	password := userpass[1] // username at index 0 is currently unused
	return password != "" && password == AccessPassword
}

// requireAuth wraps a function with another function that enforces
// HTTP Basic Auth.
func RequireAuth(handler func(conn http.ResponseWriter, req *http.Request)) func(conn http.ResponseWriter, req *http.Request) {
	return func(conn http.ResponseWriter, req *http.Request) {
		if IsAuthorized(req) {
			handler(conn, req)
		} else {
			SendUnauthorized(conn)
		}
	}
}
