// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	uploadURL = "https://winstrap.googlecode.com/files"
)

func parseNetRC() (user, pass string, err error) {
	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".netrc"))
	if err != nil {
		return
	}
	defer f.Close()
	br := bufio.NewReader(f)
	for {
		ln, err := br.ReadString('\n')
		if err != nil {
			break
		}
		f := strings.Fields(ln)
		if len(f) >= 6 && f[0] == "machine" && f[1] == "code.google.com" &&
			f[2] == "login" && f[4] == "password" {
			return f[3], f[5], nil
		}
	}
	err = errors.New("no .netrc entry found for code.google.com")
	return
}

func Upload(filename, summary string, content io.Reader) error {
	username, password, err := parseNetRC()
	if err != nil {
		return fmt.Errorf("Can't upload to code.google.com/p/winstrap; see https://code.google.com/hosting/settings to configure your ~/.netrc file. Error is: %v", err)
	}

	// Prepare upload metadata.
	var labels []string

	// Prepare multipart payload.
	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	if err := w.WriteField("summary", summary); err != nil {
		return err
	}
	for _, l := range labels {
		if err := w.WriteField("label", l); err != nil {
			return err
		}
	}
	fw, err := w.CreateFormFile("filename", filename)
	if err != nil {
		return err
	}
	if _, err = io.Copy(fw, content); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	// Send the file to Google Code.
	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return err
	}
	token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	req.Header.Set("Authorization", "Basic "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		fmt.Fprintln(os.Stderr, "upload failed")
		io.Copy(os.Stderr, resp.Body)
		return fmt.Errorf("upload: %s", resp.Status)
	}
	return nil
}
