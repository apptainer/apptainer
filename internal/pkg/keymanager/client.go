// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package keymanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
	schemeHKP   = "hkp"
	schemeHKPS  = "hkps"
)

// errUnsupportedProtocolScheme is returned when an unsupported protocol scheme is encountered.
var errUnsupportedProtocolScheme = errors.New("unsupported protocol scheme")

// ErrTLSRequired is returned when an auth token is supplied with a non-TLS BaseURL.
var ErrTLSRequired = errors.New("TLS required when auth token provided")

// normalizeURL normalizes rawURL, translating HKP/HKPS schemes to HTTP/HTTPS respectively, and
// ensures the path component is terminated with a separator.
func normalizeURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case schemeHTTP, schemeHTTPS:
		break
	case schemeHKP:
		// The HKP scheme is HTTP and implies port 11371.
		u.Scheme = schemeHTTP
		if u.Port() == "" {
			u.Host = net.JoinHostPort(u.Hostname(), "11371")
		}
	case schemeHKPS:
		// The HKPS scheme is HTTPS and implies port 443.
		u.Scheme = schemeHTTPS
	default:
		return nil, fmt.Errorf("%w %s", errUnsupportedProtocolScheme, u.Scheme)
	}

	// Ensure path is terminated with a separator, to prevent url.ResolveReference from stripping
	// the final path component of BaseURL when constructing request URL from a relative path.
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}

	return u, nil
}

// clientOptions describes the options for a Client.
type clientOptions struct {
	baseURL     string
	bearerToken string
	userAgent   string
	httpClient  *http.Client
}

// Option are used to populate co.
type Option func(co *clientOptions) error

// OptBaseURL sets the base URL of the key server to url. The supported URL schemes are "http",
// "https", "hkp", and "hkps".
func OptBaseURL(url string) Option {
	return func(co *clientOptions) error {
		co.baseURL = url
		return nil
	}
}

// OptBearerToken sets the bearer token to include in the "Authorization" header of each request.
func OptBearerToken(token string) Option {
	return func(co *clientOptions) error {
		co.bearerToken = token
		return nil
	}
}

// OptUserAgent sets the HTTP user agent to include in the "User-Agent" header of each request.
func OptUserAgent(agent string) Option {
	return func(co *clientOptions) error {
		co.userAgent = agent
		return nil
	}
}

// OptHTTPClient sets the client to use to make HTTP requests.
func OptHTTPClient(c *http.Client) Option {
	return func(co *clientOptions) error {
		co.httpClient = c
		return nil
	}
}

const defaultBaseURL = "https://keys.openpgp.org/"

// Client describes the client details.
type Client struct {
	baseURL     *url.URL     // Parsed base URL.
	bearerToken string       // Bearer token to include in "Authorization" header.
	userAgent   string       // Value to include in "User-Agent" header.
	httpClient  *http.Client // Client to use for HTTP requests.
}

// NewClient returns a Client to interact with an HKP key server according to opts.
//
// By default, the OpenPGP Key Service is used. To override this behavior, use OptBaseURL. If the
// key server requires authentication, consider using OptBearerToken.
//
// If a bearer token is specified with a non-localhost base URL that does not utilize Transport
// Layer Security (TLS), an error wrapping ErrTLSRequired is returned.
func NewClient(opts ...Option) (*Client, error) {
	co := clientOptions{
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
	}

	// Apply options.
	for _, opt := range opts {
		if err := opt(&co); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}

	c := Client{
		bearerToken: co.bearerToken,
		userAgent:   co.userAgent,
		httpClient:  co.httpClient,
	}

	// Normalize base URL.
	u, err := normalizeURL(co.baseURL)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	c.baseURL = u

	// If auth token is used, verify TLS.
	if c.bearerToken != "" && c.baseURL.Scheme != schemeHTTPS && c.baseURL.Hostname() != "localhost" {
		return nil, fmt.Errorf("%w", ErrTLSRequired)
	}

	return &c, nil
}

// NewRequest returns a new Request given a method, ref, and optional body.
//
// The context controls the entire lifetime of a request and its response: obtaining a connection,
// sending the request, and reading the response headers and body.
func (c *Client) NewRequest(ctx context.Context, method string, ref *url.URL, body io.Reader) (*http.Request, error) {
	u := c.baseURL.ResolveReference(ref)

	r, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	if v := c.bearerToken; v != "" {
		r.Header.Set("Authorization", fmt.Sprintf("BEARER %s", v))
	}

	if v := c.userAgent; v != "" {
		r.Header.Set("User-Agent", v)
	}

	return r, nil
}

// Do sends an HTTP request and returns an HTTP response.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}
