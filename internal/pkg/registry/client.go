// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

// Config contains the client configuration.

package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-log/log"
)

type Config struct {
	// Base URL of the service.
	BaseURL string
	// Auth token to include in the Authorization header of each request (if supplied).
	AuthToken string
	// User agent to include in each request (if supplied).
	UserAgent string
	// HTTPClient to use to make HTTP requests (if supplied).
	HTTPClient *http.Client
	// Logger to be used when output is generated
	Logger log.Logger
}

// DefaultConfig is a configuration that uses default values.
var DefaultConfig = &Config{}

// A Ref represents a parsed Library URI.
//
// The general form represented is:
//
//	scheme:[//host][/]path[:tags]
//
// The host contains both the hostname and port, if present. These values can be accessed using
// the Hostname and Port methods.
//
// Examples of valid URIs:
//
//  library:path:tags
//  library:/path:tags
//  library:///path:tags
//  library://host/path:tags
//  library://host:port/path:tags
//
// The tags component is a comma-separated list of one or more tags.
type Ref struct {
	Host string   // host or host:port
	Path string   // project or entity/project
	Tags []string // list of tags
}

// QuotaResponse contains quota usage and total available storage
type QuotaResponse struct {
	QuotaTotalBytes int64 `json:"quotaTotal"`
	QuotaUsageBytes int64 `json:"quotaUsage"`
}

// UploadImageComplete contains data from upload image completion
type UploadImageComplete struct {
	Quota        QuotaResponse `json:"quota"`
	ContainerURL string        `json:"containerUrl"`
}

// UploadCallback defines an interface used to perform a call-out to
// set up the source file Reader.
type UploadCallback interface {
	// Initializes the callback given a file size and source file Reader
	InitUpload(int64, io.Reader)
	// (optionally) can return a proxied Reader
	GetReader() io.Reader
	// TerminateUpload is called if the upload operation is interrupted before completion
	Terminate()
	// called when the upload operation is complete
	Finish()
}

type ClientAPI interface {
	NewClient(*Client, error)
	DeleteImage(ctx context.Context, imageRef, arch string) error
	Parse(rawRef string) (r *Ref, err error)
	UploadImage(ctx context.Context, r io.ReadSeeker, path, arch string, tags []string, description string, callback UploadCallback) (*UploadImageComplete, error)
}

// Client describes the client details.
type Client struct {
	ClientAPI
	// Base URL of the service.
	BaseURL *url.URL
	// Auth token to include in the Authorization header of each request (if supplied).
	AuthToken string
	// User agent to include in each request (if supplied).
	UserAgent string
	// HTTPClient to use to make HTTP requests.
	HTTPClient *http.Client
	// Logger to be used when output is generated
	Logger log.Logger
}

const defaultBaseURL = "https://ghcr.io/apptainer"

// NewClient sets up a new Cloud-Library Service client with the specified base URL and auth token.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig
	}

	// Determine base URL
	bu := defaultBaseURL
	if cfg.BaseURL != "" {
		bu = cfg.BaseURL
	}

	// If baseURL has a path component, ensure it is terminated with a separator, to prevent
	// url.ResolveReference from stripping the final component of the path when constructing
	// request URL.
	if !strings.HasSuffix(bu, "/") {
		bu += "/"
	}

	baseURL, err := url.Parse(bu)
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported protocol scheme %q", baseURL.Scheme)
	}

	c := &Client{
		BaseURL:   baseURL,
		AuthToken: cfg.AuthToken,
		UserAgent: cfg.UserAgent,
	}

	// Set HTTP client
	if cfg.HTTPClient != nil {
		c.HTTPClient = cfg.HTTPClient
	} else {
		c.HTTPClient = http.DefaultClient
	}

	if cfg.Logger != nil {
		c.Logger = cfg.Logger
	} else {
		c.Logger = log.DefaultLogger
	}

	return c, nil
}

// newRequest returns a new Request given a method, path (relative or
// absolute), rawQuery, and (optional) body.
func (c *Client) newRequest(method, path, rawQuery string, body io.Reader) (*http.Request, error) {
	u := c.BaseURL.ResolveReference(&url.URL{
		Path:     path,
		RawQuery: rawQuery,
	})
	r, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if v := c.AuthToken; v != "" {
		r.Header.Set("Authorization", fmt.Sprintf("BEARER %s", v))
	}
	if v := c.UserAgent; v != "" {
		r.Header.Set("User-Agent", v)
	}

	return r, nil
}
