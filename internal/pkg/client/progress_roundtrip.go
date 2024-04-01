// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package client

import (
	"io"
	"net/http"
)

const contentSizeThreshold = 1024

type RoundTripper struct {
	inner http.RoundTripper
	pb    *DownloadProgressBar
}

func NewRoundTripper(inner http.RoundTripper, pb *DownloadProgressBar) *RoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}

	rt := RoundTripper{
		inner: inner,
		pb:    pb,
	}

	return &rt
}

type rtReadCloser struct {
	inner io.ReadCloser
	pb    *DownloadProgressBar
}

func (r *rtReadCloser) Read(p []byte) (int, error) {
	return r.inner.Read(p)
}

func (r *rtReadCloser) Close() error {
	err := r.inner.Close()
	if err == nil {
		r.pb.Wait()
	} else {
		r.pb.Abort(false)
	}

	return err
}

func (t *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.pb != nil && req.Body != nil && req.ContentLength >= contentSizeThreshold {
		t.pb.Init(req.ContentLength)
		req.Body = &rtReadCloser{
			inner: t.pb.bar.ProxyReader(req.Body),
			pb:    t.pb,
		}
	}
	resp, err := t.inner.RoundTrip(req)
	if t.pb != nil && resp != nil && resp.Body != nil && resp.ContentLength >= contentSizeThreshold {
		t.pb.Init(resp.ContentLength)
		resp.Body = &rtReadCloser{
			inner: t.pb.bar.ProxyReader(resp.Body),
			pb:    t.pb,
		}
	}
	return resp, err
}
