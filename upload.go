// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	uploadURL = "https://winstrap.googlecode.com/files"
)

func Upload(filename string, content io.Reader) error {
	jsonKey, err := ioutil.ReadFile(*serviceAcctJSON)
	if err != nil {
		return err
	}
	conf, err := google.JWTConfigFromJSON(jsonKey, storage.ScopeReadWrite)
	if err != nil {
		log.Printf("Failed to get JWT config. Get a Service Account JSON token from https://console.developers.google.com/project/999119582588/apiui/credential")
		return err
	}
	httpClient := conf.Client(oauth2.NoContext)

	const maxSlurp = 1 << 20
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, content, maxSlurp)
	if err != nil && err != io.EOF {
		log.Fatalf("Error reading from stdin: %v, %v", n, err)
	}
	contentType := http.DetectContentType(buf.Bytes())

	req, err := http.NewRequest("PUT", "https://storage.googleapis.com/winstrap/"+filename, io.MultiReader(&buf, content))
	if err != nil {
		return err
	}
	req.Header.Set("x-goog-api-version", "2")
	req.Header.Set("x-goog-acl", "public-read")
	req.Header.Set("Content-Type", contentType)
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("got HTTP status %s", res.Status)
	}
	return nil
}
