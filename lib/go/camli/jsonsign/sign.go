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

package jsonsign

import (
	"bytes"
	"crypto/openpgp"
	"flag"
	"fmt"
	"io"
	"json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"camli/blobref"
	"camli/misc/gpgagent"
	"camli/misc/pinentry"
)

var _ = log.Printf

var flagSecretRing = ""

func AddFlags() {
	defSecRing := filepath.Join(os.Getenv("HOME"), ".gnupg", "secring.gpg")
	flag.StringVar(&flagSecretRing, "secret-keyring", defSecRing,
		"GnuPG secret keyring file to use.")
}

type EntityFetcher interface {
	FetchEntity(keyId string) (*openpgp.Entity, os.Error)
}

type FileEntityFetcher struct {
	File string
}

func FlagEntityFetcher() *FileEntityFetcher {
	return &FileEntityFetcher{File: flagSecretRing}
}

type CachingEntityFetcher struct {
	Fetcher EntityFetcher

	lk sync.Mutex
	m  map[string]*openpgp.Entity
}

func (ce *CachingEntityFetcher) FetchEntity(keyId string) (*openpgp.Entity, os.Error) {
	ce.lk.Lock()
	if ce.m != nil {
		e := ce.m[keyId]
		if e != nil {
			ce.lk.Unlock()
			return e, nil
		}
	}
	ce.lk.Unlock()

	e, err := ce.Fetcher.FetchEntity(keyId)
	if err == nil {
		ce.lk.Lock()
		defer ce.lk.Unlock()
		if ce.m == nil {
			ce.m = make(map[string]*openpgp.Entity)
		}
		ce.m[keyId] = e
	}

	return e, err
}

func (fe *FileEntityFetcher) FetchEntity(keyId string) (*openpgp.Entity, os.Error) {
	f, err := os.Open(fe.File)
	if err != nil {
		return nil, fmt.Errorf("jsonsign: FetchEntity: %v", err)
	}
	defer f.Close()
	el, err := openpgp.ReadKeyRing(f)
	if err != nil {
		return nil, fmt.Errorf("jsonsign: openpgp.ReadKeyRing of %q: %v", fe.File, err)
	}
	for _, e := range el {
		pubk := &e.PrivateKey.PublicKey
		if pubk.KeyIdString() != keyId {
			continue
		}
		if e.PrivateKey.Encrypted {
			if err := fe.decryptEntity(e); err == nil {
				return e, nil
			} else {
				return nil, err
			}
		}
		return e, nil
	}
	return nil, fmt.Errorf("jsonsign: entity for keyid %q not found in %q", keyId, fe.File)
}

func (fe *FileEntityFetcher) decryptEntity(e *openpgp.Entity) os.Error {
	// TODO: syscall.Mlock a region and keep pass phrase in it.
	pubk := &e.PrivateKey.PublicKey
	desc := fmt.Sprintf("Need to unlock GPG key %s to use it for signing.",
		pubk.KeyIdShortString())

	conn, err := gpgagent.NewConn()
	switch err {
	case gpgagent.ErrNoAgent:
		fmt.Fprintf(os.Stderr, "Note: gpg-agent not found; resorting to on-demand password entry.\n")
	case nil:
		defer conn.Close()
		req := &gpgagent.PassphraseRequest{
			CacheKey: "camli:jsonsign:" + pubk.KeyIdShortString(),
			Prompt:   "Passphrase",
			Desc:     desc,
		}
		for tries := 0; tries < 2; tries++ {
			pass, err := conn.GetPassphrase(req)
			if err == nil {
				err = e.PrivateKey.Decrypt([]byte(pass))
				if err == nil {
					return nil
				}
				req.Error = "Passphrase failed to decrypt: " + err.String()
				conn.RemoveFromCache(req.CacheKey)
				continue
			}
			if err == gpgagent.ErrCancel {
				return os.NewError("jsonsign: failed to decrypt key; action canceled")
			}
			log.Printf("jsonsign: gpgagent: %v", err)
		}
	default:
		log.Printf("jsonsign: gpgagent: %v", err)
	}

	pinReq := &pinentry.Request{Desc: desc, Prompt: "Passphrase"}
	for tries := 0; tries < 2; tries++ {
		pass, err := pinReq.GetPIN()
		if err == nil {
			err = e.PrivateKey.Decrypt([]byte(pass))
			if err == nil {
				return nil
			}
			pinReq.Error = "Passphrase failed to decrypt: " + err.String()
			continue
		}
		if err == pinentry.ErrCancel {
			return os.NewError("jsonsign: failed to decrypt key; action canceled")
		}
		log.Printf("jsonsign: pinentry: %v", err)
	}
	return fmt.Errorf("jsonsign: failed to decrypt key %q", pubk.KeyIdShortString())
}

type SignRequest struct {
	UnsignedJson string
	Fetcher      interface{} // blobref.Fetcher or blobref.StreamingFetcher
	ServerMode   bool        // if true, can't use pinentry or gpg-agent, etc.

	// Optional function to return an entity (including decrypting
	// the PrivateKey, if necessary)
	EntityFetcher EntityFetcher

	// SecretKeyringPath is only used if EntityFetcher is nil,
	// in which case SecretKeyringPath is used if non-empty.
	// As a final resort, the flag value (defaulting to
	// ~/.gnupg/secring.gpg) is used.
	SecretKeyringPath string
}

func (sr *SignRequest) secretRingPath() string {
	if sr.SecretKeyringPath != "" {
		return sr.SecretKeyringPath
	}
	return flagSecretRing
}

func (sr *SignRequest) Sign() (signedJson string, err os.Error) {
	trimmedJson := strings.TrimRightFunc(sr.UnsignedJson, unicode.IsSpace)

	// TODO: make sure these return different things
	inputfail := func(msg string) (string, os.Error) {
		return "", os.NewError(msg)
	}
	execfail := func(msg string) (string, os.Error) {
		return "", os.NewError(msg)
	}

	jmap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(trimmedJson), &jmap); err != nil {
		return inputfail("json parse error")
	}

	camliSigner, hasSigner := jmap["camliSigner"]
	if !hasSigner {
		return inputfail("json lacks \"camliSigner\" key with public key blobref")
	}

	camliSignerStr, _ := camliSigner.(string)
	signerBlob := blobref.Parse(camliSignerStr)
	if signerBlob == nil {
		return inputfail("json \"camliSigner\" key is malformed or unsupported")
	}

	var pubkeyReader io.ReadCloser
	switch fetcher := sr.Fetcher.(type) {
	case blobref.SeekFetcher:
		pubkeyReader, _, err = fetcher.Fetch(signerBlob)
	case blobref.StreamingFetcher:
		pubkeyReader, _, err = fetcher.FetchStreaming(signerBlob)
	default:
		panic(fmt.Sprintf("jsonsign: bogus SignRequest.Fetcher of type %T", sr.Fetcher))
	}
	if err != nil {
		// TODO: not really either an inputfail or an execfail.. but going
		// with exec for now.
		return execfail(fmt.Sprintf("failed to find public key %s", signerBlob.String()))
	}

	pubk, err := openArmoredPublicKeyFile(pubkeyReader)
	if err != nil {
		return execfail(fmt.Sprintf("failed to parse public key from blobref %s: %v", signerBlob.String(), err))
	}

	// This check should be redundant if the above JSON parse succeeded, but
	// for explicitness...
	if len(trimmedJson) == 0 || trimmedJson[len(trimmedJson)-1] != '}' {
		return inputfail("json parameter lacks trailing '}'")
	}
	trimmedJson = trimmedJson[0 : len(trimmedJson)-1]

	// sign it
	secring, err := os.Open(sr.secretRingPath())
	if err != nil {
		return "", fmt.Errorf("jsonsign: failed to open secret ring file %q: %v", sr.secretRingPath(), err)
	}
	defer secring.Close()

	entityFetcher := sr.EntityFetcher
	if entityFetcher == nil {
		file := sr.SecretKeyringPath
		if file == "" {
			file = flagSecretRing
		}
		if file == "" {
			return "", os.NewError("jsonsign: no EntityFetcher, SecretKeyringPath, or secret-keyring flag provided")
		}
		entityFetcher = &FileEntityFetcher{File: file}
	}
	signer, err := entityFetcher.FetchEntity(pubk.KeyIdString())
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = openpgp.ArmoredDetachSign(&buf, signer, strings.NewReader(trimmedJson))
	if err != nil {
		return "", err
	}

	output := buf.String()

	index1 := strings.Index(output, "\n\n")
	index2 := strings.Index(output, "\n-----")
	if index1 == -1 || index2 == -1 {
		return execfail("Failed to parse signature from gpg.")
	}
	inner := output[index1+2 : index2]
	signature := strings.Replace(inner, "\n", "", -1)

	return fmt.Sprintf("%s,\"camliSig\":\"%s\"}\n", trimmedJson, signature), nil
}
