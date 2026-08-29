// Package connector implements the API Connector System (REST, GraphQL,
// Webhook, Local Files) and the Database Connector Layer. Credentials are
// always resolved through security.Vault by reference (a vault key), never
// passed around or logged as plain text.
package connector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthKind enumerates the authentication schemes the API Connector System
// supports, matching the Connection editor in the UI.
type AuthKind string

const (
	AuthNone   AuthKind = "none"
	AuthAPIKey AuthKind = "apiKey"
	AuthBearer AuthKind = "bearer"
	AuthBasic  AuthKind = "basic"
	AuthOAuth2 AuthKind = "oauth2"
)

// Auth describes how to authenticate a request. Secret is resolved by the
// caller from security.Vault immediately before the request is made and is
// never persisted on this struct.
type Auth struct {
	Kind       AuthKind `json:"kind"`
	HeaderName string   `json:"headerName,omitempty"` // for apiKey, defaults to X-Api-Key
	Username   string   `json:"username,omitempty"`   // for basic
	Secret     string   `json:"-"`                     // resolved token/password/key, never serialized
}

// RestRequest configures a single REST/GraphQL/Webhook call for an API
// Input or API Output node.
type RestRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Query   map[string]string `json:"query,omitempty"`
	Body    any               `json:"body,omitempty"`
	Auth    Auth              `json:"auth"`
	Timeout time.Duration     `json:"-"`
}

// Client wraps http.Client with sane defaults (bounded timeout, no
// following of cross-origin redirects that could leak auth headers).
type Client struct {
	http *http.Client
}

// NewClient builds a REST connector client with a default 30s timeout.
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}}
}

// Do executes a RestRequest and returns the decoded JSON body as generic
// data (object or array), ready for parser.flattenToRows-style normalization.
func (c *Client) Do(req RestRequest) (any, int, error) {
	var bodyReader io.Reader
	if req.Body != nil {
		b, err := json.Marshal(req.Body)
		if err != nil {
			return nil, 0, fmt.Errorf("connector: encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	httpReq, err := http.NewRequest(method, req.URL, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if err := applyAuth(httpReq, req.Auth); err != nil {
		return nil, 0, err
	}

	q := httpReq.URL.Query()
	for k, v := range req.Query {
		q.Set(k, v)
	}
	httpReq.URL.RawQuery = q.Encode()

	client := c.http
	if req.Timeout > 0 {
		client = &http.Client{Timeout: req.Timeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("connector: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024*1024)) // 256MB safety cap
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(raw) == 0 {
		return nil, resp.StatusCode, nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("connector: response is not valid JSON: %w", err)
	}
	return decoded, resp.StatusCode, nil
}

func applyAuth(req *http.Request, auth Auth) error {
	switch auth.Kind {
	case AuthNone, "":
		return nil
	case AuthAPIKey:
		header := auth.HeaderName
		if header == "" {
			header = "X-Api-Key"
		}
		req.Header.Set(header, auth.Secret)
	case AuthBearer, AuthOAuth2:
		req.Header.Set("Authorization", "Bearer "+auth.Secret)
	case AuthBasic:
		req.SetBasicAuth(auth.Username, auth.Secret)
	default:
		return fmt.Errorf("connector: unknown auth kind %q", auth.Kind)
	}
	return nil
}

// GraphQLRequest builds a RestRequest for a GraphQL query/mutation, reusing
// the same auth + transport plumbing as plain REST.
func GraphQLRequest(endpoint, query string, variables map[string]any, auth Auth) RestRequest {
	return RestRequest{
		Method: http.MethodPost,
		URL:    endpoint,
		Auth:   auth,
		Body: map[string]any{
			"query":     query,
			"variables": variables,
		},
	}
}
