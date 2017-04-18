// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-querystring/query"
)

const (
	// StatusUnprocessableEntity is the status code returned when sending a request with invalid fields.
	StatusUnprocessableEntity = 422
)

const (
	libraryVersion = "0.1"
	defaultBaseURL = "https://api.github.com/"
	uploadBaseURL  = "https://uploads.github.com/"
	userAgent      = "go-github/" + libraryVersion

	headerRateLimit     = "X-RateLimit-Limit"
	headerRateRemaining = "X-RateLimit-Remaining"
	headerRateReset     = "X-RateLimit-Reset"
	headerOTP           = "X-GitHub-OTP"

	mediaTypeV3      = "application/vnd.github.v3+json"
	defaultMediaType = "application/octet-stream"
	mediaTypeV3SHA   = "application/vnd.github.v3.sha"

	// Media Type values to access preview APIs

	// https://developer.github.com/changes/2015-03-09-licenses-api/
	mediaTypeLicensesPreview = "application/vnd.github.drax-preview+json"

	// https://developer.github.com/changes/2014-12-09-new-attributes-for-stars-api/
	mediaTypeStarringPreview = "application/vnd.github.v3.star+json"

	// https://developer.github.com/changes/2015-06-24-api-enhancements-for-working-with-organization-permissions/
	mediaTypeOrgPermissionPreview     = "application/vnd.github.ironman-preview+json"
	mediaTypeOrgPermissionRepoPreview = "application/vnd.github.ironman-preview.repository+json"

	// https://developer.github.com/changes/2015-11-11-protected-branches-api/
	mediaTypeProtectedBranchesPreview = "application/vnd.github.loki-preview+json"

	// https://developer.github.com/changes/2016-02-11-issue-locking-api/
	mediaTypeIssueLockingPreview = "application/vnd.github.the-key-preview+json"

	// https://help.github.com/enterprise/2.4/admin/guides/migrations/exporting-the-github-com-organization-s-repositories/
	mediaTypeMigrationsPreview = "application/vnd.github.wyandotte-preview+json"

	// https://developer.github.com/changes/2016-04-06-deployment-and-deployment-status-enhancements/
	mediaTypeDeploymentStatusPreview = "application/vnd.github.ant-man-preview+json"

	// https://developer.github.com/changes/2016-02-19-source-import-preview-api/
	mediaTypeImportPreview = "application/vnd.github.barred-rock-preview"

	// https://developer.github.com/changes/2016-05-12-reactions-api-preview/
	mediaTypeReactionsPreview = "application/vnd.github.squirrel-girl-preview"
)

// A Client manages communication with the GitHub API.
type Client struct {
	// HTTP client used to communicate with the API.
	client *http.Client
	// clientMu protects the client during calls that modify the CheckRedirect func.
	clientMu sync.Mutex

	// Base URL for API requests.  Defaults to the public GitHub API, but can be
	// set to a domain endpoint to use with GitHub Enterprise.  BaseURL should
	// always be specified with a trailing slash.
	BaseURL *url.URL

	// Base URL for uploading files.
	UploadURL *url.URL

	// User agent used when communicating with the GitHub API.
	UserAgent string

	rateMu     sync.Mutex
	rateLimits [categories]Rate // Rate limits for the client as determined by the most recent API calls.
	mostRecent rateLimitCategory

	// Services used for talking to different parts of the GitHub API.
	Activity       *ActivityService
	Authorizations *AuthorizationsService
	Gists          *GistsService
	Git            *GitService
	Gitignores     *GitignoresService
	Issues         *IssuesService
	Organizations  *OrganizationsService
	PullRequests   *PullRequestsService
	Repositories   *RepositoriesService
	Search         *SearchService
	Users          *UsersService
	Licenses       *LicensesService
	Migrations     *MigrationService
	Reactions      *ReactionsService
}

// ListOptions specifies the optional parameters to various List methods that
// support pagination.
type ListOptions struct {
	// For paginated result sets, page of results to retrieve.
	Page int `url:"page,omitempty"`

	// For paginated result sets, the number of results to include per page.
	PerPage int `url:"per_page,omitempty"`
}

// UploadOptions specifies the parameters to methods that support uploads.
type UploadOptions struct {
	Name string `url:"name,omitempty"`
}

// addOptions adds the parameters in opt as URL query parameters to s.  opt
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opt interface{}) (string, error) {
	v := reflect.ValueOf(opt)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opt)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

// NewClient returns a new GitHub API client.  If a nil httpClient is
// provided, http.DefaultClient will be used.  To use API methods which require
// authentication, provide an http.Client that will perform the authentication
// for you (such as that provided by the golang.org/x/oauth2 library).
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	baseURL, _ := url.Parse(defaultBaseURL)
	uploadURL, _ := url.Parse(uploadBaseURL)

	c := &Client{client: httpClient, BaseURL: baseURL, UserAgent: userAgent, UploadURL: uploadURL}
	c.Activity = &ActivityService{client: c}
	c.Authorizations = &AuthorizationsService{client: c}
	c.Gists = &GistsService{client: c}
	c.Git = &GitService{client: c}
	c.Gitignores = &GitignoresService{client: c}
	c.Issues = &IssuesService{client: c}
	c.Organizations = &OrganizationsService{client: c}
	c.PullRequests = &PullRequestsService{client: c}
	c.Repositories = &RepositoriesService{client: c}
	c.Search = &SearchService{client: c}
	c.Users = &UsersService{client: c}
	c.Licenses = &LicensesService{client: c}
	c.Migrations = &MigrationService{client: c}
	c.Reactions = &ReactionsService{client: c}
	return c
}

// NewRequest creates an API request. A relative URL can be provided in urlStr,
// in which case it is resolved relative to the BaseURL of the Client.
// Relative URLs should always be specified without a preceding slash.  If
// specified, the value pointed to by body is JSON encoded and included as the
// request body.
func (c *Client) NewRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.ResolveReference(rel)

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", mediaTypeV3)
	if c.UserAgent != "" {
		req.Header.Add("User-Agent", c.UserAgent)
	}
	return req, nil
}

// NewUploadRequest creates an upload request. A relative URL can be provided in
// urlStr, in which case it is resolved relative to the UploadURL of the Client.
// Relative URLs should always be specified without a preceding slash.
func (c *Client) NewUploadRequest(urlStr string, reader io.Reader, size int64, mediaType string) (*http.Request, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	u := c.UploadURL.ResolveReference(rel)
	req, err := http.NewRequest("POST", u.String(), reader)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size

	if len(mediaType) == 0 {
		mediaType = defaultMediaType
	}
	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaTypeV3)
	req.Header.Add("User-Agent", c.UserAgent)
	return req, nil
}

// Response is a GitHub API response.  This wraps the standard http.Response
// returned from GitHub and provides convenient access to things like
// pagination links.
type Response struct {
	*http.Response

	// These fields provide the page values for paginating through a set of
	// results.  Any or all of these may be set to the zero value for
	// responses that are not part of a paginated set, or for which there
	// are no additional pages.

	NextPage  int
	PrevPage  int
	FirstPage int
	LastPage  int

	Rate
}

// newResponse creates a new Response for the provided http.Response.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	response.populatePageValues()
	response.Rate = parseRate(r)
	return response
}

// populatePageValues parses the HTTP Link response headers and populates the
// various pagination link values in the Response.
func (r *Response) populatePageValues() {
	if links, ok := r.Response.Header["Link"]; ok && len(links) > 0 {
		for _, link := range strings.Split(links[0], ",") {
			segments := strings.Split(strings.TrimSpace(link), ";")

			// link must at least have href and rel
			if len(segments) < 2 {
				continue
			}

			// ensure href is properly formatted
			if !strings.HasPrefix(segments[0], "<") || !strings.HasSuffix(segments[0], ">") {
				continue
			}

			// try to pull out page parameter
			url, err := url.Parse(segments[0][1 : len(segments[0])-1])
			if err != nil {
				continue
			}
			page := url.Query().Get("page")
			if page == "" {
				continue
			}

			for _, segment := range segments[1:] {
				switch strings.TrimSpace(segment) {
				case `rel="next"`:
					r.NextPage, _ = strconv.Atoi(page)
				case `rel="prev"`:
					r.PrevPage, _ = strconv.Atoi(page)
				case `rel="first"`:
					r.FirstPage, _ = strconv.Atoi(page)
				case `rel="last"`:
					r.LastPage, _ = strconv.Atoi(page)
				}

			}
		}
	}
}

// parseRate parses the rate related headers.
func parseRate(r *http.Response) Rate {
	var rate Rate
	if limit := r.Header.Get(headerRateLimit); limit != "" {
		rate.Limit, _ = strconv.Atoi(limit)
	}
	if remaining := r.Header.Get(headerRateRemaining); remaining != "" {
		rate.Remaining, _ = strconv.Atoi(remaining)
	}
	if reset := r.Header.Get(headerRateReset); reset != "" {
		if v, _ := strconv.ParseInt(reset, 10, 64); v != 0 {
			rate.Reset = Timestamp{time.Unix(v, 0)}
		}
	}
	return rate
}

// Rate specifies the current rate limit for the client as determined by the
// most recent API call.  If the client is used in a multi-user application,
// this rate may not always be up-to-date.
//
// Deprecated: Use the Response.Rate returned from most recent API call instead.
// Call RateLimits() to check the current rate.
func (c *Client) Rate() Rate {
	c.rateMu.Lock()
	rate := c.rateLimits[c.mostRecent]
	c.rateMu.Unlock()
	return rate
}

// Do sends an API request and returns the API response.  The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred.  If v implements the io.Writer
// interface, the raw response body will be written to v, without attempting to
// first decode it.  If rate limit is exceeded and reset time is in the future,
// Do returns *RateLimitError immediately without making a network API call.
func (c *Client) Do(req *http.Request, v interface{}) (*Response, error) {
	rateLimitCategory := category(req.URL.Path)

	// If we've hit rate limit, don't make further requests before Reset time.
	if err := c.checkRateLimitBeforeDo(req, rateLimitCategory); err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		// Drain up to 512 bytes and close the body to let the Transport reuse the connection
		io.CopyN(ioutil.Discard, resp.Body, 512)
		resp.Body.Close()
	}()

	response := newResponse(resp)

	c.rateMu.Lock()
	c.rateLimits[rateLimitCategory] = response.Rate
	c.mostRecent = rateLimitCategory
	c.rateMu.Unlock()

	err = CheckResponse(resp)
	if err != nil {
		// even though there was an error, we still return the response
		// in case the caller wants to inspect it further
		return response, err
	}

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err == io.EOF {
				err = nil // ignore EOF errors caused by empty response body
			}
		}
	}

	return response, err
}

// checkRateLimitBeforeDo does not make any network calls, but uses existing knowledge from
// current client state in order to quickly check if *RateLimitError can be immediately returned
// from Client.Do, and if so, returns it so that Client.Do can skip making a network API call unneccessarily.
// Otherwise it returns nil, and Client.Do should proceed normally.
func (c *Client) checkRateLimitBeforeDo(req *http.Request, rateLimitCategory rateLimitCategory) error {
	c.rateMu.Lock()
	rate := c.rateLimits[rateLimitCategory]
	c.rateMu.Unlock()
	if !rate.Reset.Time.IsZero() && rate.Remaining == 0 && time.Now().Before(rate.Reset.Time) {
		// Create a fake response.
		resp := &http.Response{
			Status:     http.StatusText(http.StatusForbidden),
			StatusCode: http.StatusForbidden,
			Request:    req,
			Header:     make(http.Header),
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}
		return &RateLimitError{
			Rate:     rate,
			Response: resp,
			Message:  fmt.Sprintf("API rate limit of %v still exceeded until %v, not making remote request.", rate.Limit, rate.Reset.Time),
		}
	}

	return nil
}

/*
An ErrorResponse reports one or more errors caused by an API request.

GitHub API docs: http://developer.github.com/v3/#client-errors
*/
type ErrorResponse struct {
	Response *http.Response // HTTP response that caused this error
	Message  string         `json:"message"` // error message
	Errors   []Error        `json:"errors"`  // more detail on individual errors
	// Block is only populated on certain types of errors such as code 451.
	// See https://developer.github.com/changes/2016-03-17-the-451-status-code-is-now-supported/
	// for more information.
	Block *struct {
		Reason    string     `json:"reason,omitempty"`
		CreatedAt *Timestamp `json:"created_at,omitempty"`
	} `json:"block,omitempty"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %v %+v",
		r.Response.Request.Method, sanitizeURL(r.Response.Request.URL),
		r.Response.StatusCode, r.Message, r.Errors)
}

// TwoFactorAuthError occurs when using HTTP Basic Authentication for a user
// that has two-factor authentication enabled.  The request can be reattempted
// by providing a one-time password in the request.
type TwoFactorAuthError ErrorResponse

func (r *TwoFactorAuthError) Error() string { return (*ErrorResponse)(r).Error() }

// RateLimitError occurs when GitHub returns 403 Forbidden response with a rate limit
// remaining value of 0, and error message starts with "API rate limit exceeded for ".
type RateLimitError struct {
	Rate     Rate           // Rate specifies last known rate limit for the client
	Response *http.Response // HTTP response that caused this error
	Message  string         `json:"message"` // error message
}

func (r *RateLimitError) Error() string {
	return fmt.Sprintf("%v %v: %d %v; rate reset in %v",
		r.Response.Request.Method, sanitizeURL(r.Response.Request.URL),
		r.Response.StatusCode, r.Message, r.Rate.Reset.Time.Sub(time.Now()))
}

// sanitizeURL redacts the client_secret parameter from the URL which may be
// exposed to the user, specifically in the ErrorResponse error message.
func sanitizeURL(uri *url.URL) *url.URL {
	if uri == nil {
		return nil
	}
	params := uri.Query()
	if len(params.Get("client_secret")) > 0 {
		params.Set("client_secret", "REDACTED")
		uri.RawQuery = params.Encode()
	}
	return uri
}

/*
An Error reports more details on an individual error in an ErrorResponse.
These are the possible validation error codes:

    missing:
        resource does not exist
    missing_field:
        a required field on a resource has not been set
    invalid:
        the formatting of a field is invalid
    already_exists:
        another resource has the same valid as this field

GitHub API docs: http://developer.github.com/v3/#client-errors
*/
type Error struct {
	Resource string `json:"resource"` // resource on which the error occurred
	Field    string `json:"field"`    // field on which the error occurred
	Code     string `json:"code"`     // validation error code
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v error caused by %v field on %v resource",
		e.Code, e.Field, e.Resource)
}

// CheckResponse checks the API response for errors, and returns them if
// present.  A response is considered an error if it has a status code outside
// the 200 range.  API error responses are expected to have either no response
// body, or a JSON response body that maps to ErrorResponse.  Any other
// response body will be silently ignored.
//
// The error type will be *RateLimitError for rate limit exceeded errors,
// and *TwoFactorAuthError for two-factor authentication errors.
func CheckResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}
	errorResponse := &ErrorResponse{Response: r}
	data, err := ioutil.ReadAll(r.Body)
	if err == nil && data != nil {
		json.Unmarshal(data, errorResponse)
	}
	switch {
	case r.StatusCode == http.StatusUnauthorized && strings.HasPrefix(r.Header.Get(headerOTP), "required"):
		return (*TwoFactorAuthError)(errorResponse)
	case r.StatusCode == http.StatusForbidden && r.Header.Get(headerRateRemaining) == "0" && strings.HasPrefix(errorResponse.Message, "API rate limit exceeded for "):
		return &RateLimitError{
			Rate:     parseRate(r),
			Response: errorResponse.Response,
			Message:  errorResponse.Message,
		}
	default:
		return errorResponse
	}
}

// parseBoolResponse determines the boolean result from a GitHub API response.
// Several GitHub API methods return boolean responses indicated by the HTTP
// status code in the response (true indicated by a 204, false indicated by a
// 404).  This helper function will determine that result and hide the 404
// error if present.  Any other error will be returned through as-is.
func parseBoolResponse(err error) (bool, error) {
	if err == nil {
		return true, nil
	}

	if err, ok := err.(*ErrorResponse); ok && err.Response.StatusCode == http.StatusNotFound {
		// Simply false.  In this one case, we do not pass the error through.
		return false, nil
	}

	// some other real error occurred
	return false, err
}

// Rate represents the rate limit for the current client.
type Rate struct {
	// The number of requests per hour the client is currently limited to.
	Limit int `json:"limit"`

	// The number of remaining requests the client can make this hour.
	Remaining int `json:"remaining"`

	// The time at which the current rate limit will reset.
	Reset Timestamp `json:"reset"`
}

func (r Rate) String() string {
	return Stringify(r)
}

// RateLimits represents the rate limits for the current client.
type RateLimits struct {
	// The rate limit for non-search API requests.  Unauthenticated
	// requests are limited to 60 per hour.  Authenticated requests are
	// limited to 5,000 per hour.
	//
	// GitHub API docs: https://developer.github.com/v3/#rate-limiting
	Core *Rate `json:"core"`

	// The rate limit for search API requests.  Unauthenticated requests
	// are limited to 5 requests per minutes.  Authenticated requests are
	// limited to 20 per minute.
	//
	// GitHub API docs: https://developer.github.com/v3/search/#rate-limit
	Search *Rate `json:"search"`
}

func (r RateLimits) String() string {
	return Stringify(r)
}

type rateLimitCategory uint8

const (
	coreCategory rateLimitCategory = iota
	searchCategory

	categories // An array of this length will be able to contain all rate limit categories.
)

// category returns the rate limit category of the endpoint, determined by Request.URL.Path.
func category(path string) rateLimitCategory {
	switch {
	default:
		return coreCategory
	case strings.HasPrefix(path, "/search/"):
		return searchCategory
	}
}

// Deprecated: RateLimit is deprecated, use RateLimits instead.
func (c *Client) RateLimit() (*Rate, *Response, error) {
	limits, resp, err := c.RateLimits()
	if limits == nil {
		return nil, nil, err
	}

	return limits.Core, resp, err
}

// RateLimits returns the rate limits for the current client.
func (c *Client) RateLimits() (*RateLimits, *Response, error) {
	req, err := c.NewRequest("GET", "rate_limit", nil)
	if err != nil {
		return nil, nil, err
	}

	response := new(struct {
		Resources *RateLimits `json:"resources"`
	})
	resp, err := c.Do(req, response)
	if err != nil {
		return nil, nil, err
	}

	if response.Resources != nil {
		c.rateMu.Lock()
		if response.Resources.Core != nil {
			c.rateLimits[coreCategory] = *response.Resources.Core
		}
		if response.Resources.Search != nil {
			c.rateLimits[searchCategory] = *response.Resources.Search
		}
		c.rateMu.Unlock()
	}

	return response.Resources, resp, err
}

/*
UnauthenticatedRateLimitedTransport allows you to make unauthenticated calls
that need to use a higher rate limit associated with your OAuth application.

	t := &github.UnauthenticatedRateLimitedTransport{
		ClientID:     "your app's client ID",
		ClientSecret: "your app's client secret",
	}
	client := github.NewClient(t.Client())

This will append the querystring params client_id=xxx&client_secret=yyy to all
requests.

See http://developer.github.com/v3/#unauthenticated-rate-limited-requests for
more information.
*/
type UnauthenticatedRateLimitedTransport struct {
	// ClientID is the GitHub OAuth client ID of the current application, which
	// can be found by selecting its entry in the list at
	// https://github.com/settings/applications.
	ClientID string

	// ClientSecret is the GitHub OAuth client secret of the current
	// application.
	ClientSecret string

	// Transport is the underlying HTTP transport to use when making requests.
	// It will default to http.DefaultTransport if nil.
	Transport http.RoundTripper
}

// RoundTrip implements the RoundTripper interface.
func (t *UnauthenticatedRateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.ClientID == "" {
		return nil, errors.New("t.ClientID is empty")
	}
	if t.ClientSecret == "" {
		return nil, errors.New("t.ClientSecret is empty")
	}

	// To set extra querystring params, we must make a copy of the Request so
	// that we don't modify the Request we were given. This is required by the
	// specification of http.RoundTripper.
	req = cloneRequest(req)
	q := req.URL.Query()
	q.Set("client_id", t.ClientID)
	q.Set("client_secret", t.ClientSecret)
	req.URL.RawQuery = q.Encode()

	// Make the HTTP request.
	return t.transport().RoundTrip(req)
}

// Client returns an *http.Client that makes requests which are subject to the
// rate limit of your OAuth application.
func (t *UnauthenticatedRateLimitedTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func (t *UnauthenticatedRateLimitedTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

// BasicAuthTransport is an http.RoundTripper that authenticates all requests
// using HTTP Basic Authentication with the provided username and password.  It
// additionally supports users who have two-factor authentication enabled on
// their GitHub account.
type BasicAuthTransport struct {
	Username string // GitHub username
	Password string // GitHub password
	OTP      string // one-time password for users with two-factor auth enabled

	// Transport is the underlying HTTP transport to use when making requests.
	// It will default to http.DefaultTransport if nil.
	Transport http.RoundTripper
}

// RoundTrip implements the RoundTripper interface.
func (t *BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = cloneRequest(req) // per RoundTrip contract
	req.SetBasicAuth(t.Username, t.Password)
	if t.OTP != "" {
		req.Header.Add(headerOTP, t.OTP)
	}
	return t.transport().RoundTrip(req)
}

// Client returns an *http.Client that makes requests that are authenticated
// using HTTP Basic Authentication.
func (t *BasicAuthTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func (t *BasicAuthTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

// cloneRequest returns a clone of the provided *http.Request. The clone is a
// shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

// Bool is a helper routine that allocates a new bool value
// to store v and returns a pointer to it.
func Bool(v bool) *bool { return &v }

// Int is a helper routine that allocates a new int value
// to store v and returns a pointer to it.
func Int(v int) *int { return &v }

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it.
func String(v string) *string { return &v }
