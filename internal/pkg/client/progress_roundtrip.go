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
	"context"
	"net/http"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/vbauerster/mpb/v8"
	"golang.org/x/term"
)

const contentSizeThreshold = 64 * 1024

type RoundTripper struct {
	inner http.RoundTripper
	p     *mpb.Progress
	bars  []*mpb.Bar
	sizes []int64
}

func NewRoundTripper(ctx context.Context, inner http.RoundTripper) *RoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}

	rt := RoundTripper{
		inner: inner,
	}

	if term.IsTerminal(2) && sylog.GetLevel() >= 0 {
		rt.p = mpb.NewWithContext(ctx)
	}

	return &rt
}

func (t *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.p == nil || req.Method != http.MethodGet {
		return t.inner.RoundTrip(req)
	}

	resp, err := t.inner.RoundTrip(req)
	if resp != nil && resp.Body != nil && resp.ContentLength >= contentSizeThreshold {
		bar := t.p.AddBar(resp.ContentLength, defaultOption...)
		t.bars = append(t.bars, bar)
		t.sizes = append(t.sizes, resp.ContentLength)
		resp.Body = bar.ProxyReader(resp.Body)
	}
	return resp, err
}

// ProgressComplete overrides all progress bars, setting them to 100% complete.
func (t *RoundTripper) ProgressComplete() {
	if t.p != nil {
		for i, bar := range t.bars {
			bar.SetCurrent(t.sizes[i])
		}
	}
}

// ProgressWait shuts down the mpb Progress container by waiting for all bars to
// complete.
func (t *RoundTripper) ProgressWait() {
	if t.p != nil {
		t.p.Wait()
	}
}

// ProgressShutdown immediately shuts down the mpb Progress container.
func (t *RoundTripper) ProgressShutdown() {
	if t.p != nil {
		t.p.Shutdown()
	}
}
