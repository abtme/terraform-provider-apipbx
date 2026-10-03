// Package client is a minimal apipbx REST API client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

func New(base, token string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), token: token, http: &http.Client{}}
}

// ErrNotFound is returned (wrapped) when the API answers 404.
var ErrNotFound = errors.New("not found")

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		msg := fmt.Errorf("%s %s: %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: %w", ErrNotFound, msg)
		}
		return msg
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func create[T any](ctx context.Context, c *Client, path string, in any) (T, error) {
	var out T
	err := c.do(ctx, "POST", path, in, &out)
	return out, err
}

func get[T any](ctx context.Context, c *Client, path string) (T, error) {
	var out T
	err := c.do(ctx, "GET", path, nil, &out)
	return out, err
}

func patch[T any](ctx context.Context, c *Client, path string, in any) (T, error) {
	var out T
	err := c.do(ctx, "PATCH", path, in, &out)
	return out, err
}

func (c *Client) remove(ctx context.Context, path string) error {
	return c.do(ctx, "DELETE", path, nil, nil)
}

func tp(tenant int64, coll string) string { return fmt.Sprintf("/v1/tenants/%d/%s", tenant, coll) }

// ---- tenants

type Tenant struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (c *Client) CreateTenant(ctx context.Context, slug, name string) (Tenant, error) {
	return create[Tenant](ctx, c, "/v1/tenants", map[string]string{"slug": slug, "name": name})
}

func (c *Client) GetTenant(ctx context.Context, id int64) (Tenant, error) {
	return get[Tenant](ctx, c, fmt.Sprintf("/v1/tenants/%d", id))
}

func (c *Client) DeleteTenant(ctx context.Context, id int64) error {
	return c.remove(ctx, fmt.Sprintf("/v1/tenants/%d", id))
}

// ---- API keys

type APIKey struct {
	ID       int64  `json:"id"`
	TenantID *int64 `json:"tenant_id"`
	Name     string `json:"name"`
	// Key is the secret; the API returns it once, on creation.
	Key string `json:"key"`
}

func (c *Client) CreateAPIKey(ctx context.Context, name string, tenantID *int64) (APIKey, error) {
	return create[APIKey](ctx, c, "/v1/api-keys", map[string]any{"name": name, "tenant_id": tenantID})
}

// GetAPIKey finds a key by id; there is no single-key endpoint.
func (c *Client) GetAPIKey(ctx context.Context, id int64) (APIKey, error) {
	keys, err := get[[]APIKey](ctx, c, "/v1/api-keys")
	if err != nil {
		return APIKey{}, err
	}
	for _, k := range keys {
		if k.ID == id {
			return k, nil
		}
	}
	return APIKey{}, fmt.Errorf("%w: api key %d", ErrNotFound, id)
}

func (c *Client) DeleteAPIKey(ctx context.Context, id int64) error {
	return c.remove(ctx, fmt.Sprintf("/v1/api-keys/%d", id))
}

// ---- extensions

type Extension struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	Number      string `json:"number"`
	Name        string `json:"name"`
	Tech        string `json:"tech"`
	Secret      string `json:"secret"`
	Username    string `json:"username"`
	OutboundCID string `json:"outbound_cid"`
	RingTime    int    `json:"ring_time"`
	Enabled     bool   `json:"enabled"`
}

type ExtensionInput struct {
	Number      string `json:"number"`
	Name        string `json:"name"`
	Tech        string `json:"tech,omitempty"`
	Secret      string `json:"secret,omitempty"` // generated when empty
	OutboundCID string `json:"outbound_cid"`
	RingTime    int    `json:"ring_time,omitempty"`
}

type ExtensionPatch struct {
	Name        *string `json:"name,omitempty"`
	Secret      *string `json:"secret,omitempty"`
	OutboundCID *string `json:"outbound_cid,omitempty"`
	RingTime    *int    `json:"ring_time,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

func (c *Client) CreateExtension(ctx context.Context, tenant int64, in ExtensionInput) (Extension, error) {
	return create[Extension](ctx, c, tp(tenant, "extensions"), in)
}

func (c *Client) GetExtension(ctx context.Context, tenant, id int64) (Extension, error) {
	return get[Extension](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "extensions"), id))
}

func (c *Client) UpdateExtension(ctx context.Context, tenant, id int64, p ExtensionPatch) (Extension, error) {
	return patch[Extension](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "extensions"), id), p)
}

func (c *Client) DeleteExtension(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "extensions"), id))
}

// ---- trunks

type Trunk struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	Name           string `json:"name"`
	AsteriskID     string `json:"asterisk_id"`
	Tech           string `json:"tech"`
	Host           string `json:"host"`
	Port           *int   `json:"port"`
	Username       string `json:"username"`
	Secret         string `json:"secret"`
	Register       bool   `json:"register"`
	OutboundCID    string `json:"outbound_cid"`
	MaxChannels    int    `json:"max_channels"`
	ContinueOnFail bool   `json:"continue_on_fail"`
	Enabled        bool   `json:"enabled"`
}

type TrunkInput struct {
	Name           string `json:"name"`
	Tech           string `json:"tech"`
	Host           string `json:"host"`
	Port           *int   `json:"port,omitempty"`
	Username       string `json:"username"`
	Secret         string `json:"secret"`
	Register       bool   `json:"register"`
	OutboundCID    string `json:"outbound_cid"`
	MaxChannels    int    `json:"max_channels"`
	ContinueOnFail bool   `json:"continue_on_fail"`
}

type TrunkPatch struct {
	Host           *string `json:"host,omitempty"`
	Port           *int    `json:"port,omitempty"`
	Username       *string `json:"username,omitempty"`
	Secret         *string `json:"secret,omitempty"`
	Register       *bool   `json:"register,omitempty"`
	OutboundCID    *string `json:"outbound_cid,omitempty"`
	MaxChannels    *int    `json:"max_channels,omitempty"`
	ContinueOnFail *bool   `json:"continue_on_fail,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

func (c *Client) CreateTrunk(ctx context.Context, tenant int64, in TrunkInput) (Trunk, error) {
	return create[Trunk](ctx, c, tp(tenant, "trunks"), in)
}

func (c *Client) GetTrunk(ctx context.Context, tenant, id int64) (Trunk, error) {
	return get[Trunk](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "trunks"), id))
}

func (c *Client) UpdateTrunk(ctx context.Context, tenant, id int64, p TrunkPatch) (Trunk, error) {
	return patch[Trunk](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "trunks"), id), p)
}

func (c *Client) DeleteTrunk(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "trunks"), id))
}

// ---- ring groups

type Destination struct {
	Type string `json:"type"`
	ID   *int64 `json:"id"`
}

type RingGroup struct {
	ID       int64       `json:"id"`
	TenantID int64       `json:"tenant_id"`
	Number   string      `json:"number"`
	Name     string      `json:"name"`
	Strategy string      `json:"strategy"`
	RingTime int         `json:"ring_time"`
	Members  []int64     `json:"members"`
	Failover Destination `json:"failover"`
}

type RingGroupInput struct {
	Number   string       `json:"number"`
	Name     string       `json:"name"`
	Strategy string       `json:"strategy,omitempty"`
	RingTime int          `json:"ring_time,omitempty"`
	Members  []int64      `json:"members"`
	Failover *Destination `json:"failover,omitempty"`
}

type RingGroupPatch struct {
	Name     *string      `json:"name,omitempty"`
	Strategy *string      `json:"strategy,omitempty"`
	RingTime *int         `json:"ring_time,omitempty"`
	Members  *[]int64     `json:"members,omitempty"`
	Failover *Destination `json:"failover,omitempty"`
}

func (c *Client) CreateRingGroup(ctx context.Context, tenant int64, in RingGroupInput) (RingGroup, error) {
	return create[RingGroup](ctx, c, tp(tenant, "ring-groups"), in)
}

func (c *Client) GetRingGroup(ctx context.Context, tenant, id int64) (RingGroup, error) {
	return get[RingGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "ring-groups"), id))
}

func (c *Client) UpdateRingGroup(ctx context.Context, tenant, id int64, p RingGroupPatch) (RingGroup, error) {
	return patch[RingGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "ring-groups"), id), p)
}

func (c *Client) DeleteRingGroup(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "ring-groups"), id))
}

// ---- inbound routes (addressed by DID)

type InboundRoute struct {
	DID         string      `json:"did"`
	TenantID    int64       `json:"tenant_id"`
	Description string      `json:"description"`
	CIDPrefix   string      `json:"cid_prefix"`
	Destination Destination `json:"destination"`
}

type InboundRouteInput struct {
	DID         string      `json:"did"`
	Description string      `json:"description"`
	CIDPrefix   string      `json:"cid_prefix"`
	Destination Destination `json:"destination"`
}

type InboundRoutePatch struct {
	Description *string      `json:"description,omitempty"`
	CIDPrefix   *string      `json:"cid_prefix,omitempty"`
	Destination *Destination `json:"destination,omitempty"`
}

func inboundPath(tenant int64, did string) string {
	return tp(tenant, "inbound-routes") + "/" + url.PathEscape(did)
}

func (c *Client) CreateInboundRoute(ctx context.Context, tenant int64, in InboundRouteInput) (InboundRoute, error) {
	return create[InboundRoute](ctx, c, tp(tenant, "inbound-routes"), in)
}

func (c *Client) GetInboundRoute(ctx context.Context, tenant int64, did string) (InboundRoute, error) {
	return get[InboundRoute](ctx, c, inboundPath(tenant, did))
}

func (c *Client) UpdateInboundRoute(ctx context.Context, tenant int64, did string, p InboundRoutePatch) (InboundRoute, error) {
	return patch[InboundRoute](ctx, c, inboundPath(tenant, did), p)
}

func (c *Client) DeleteInboundRoute(ctx context.Context, tenant int64, did string) error {
	return c.remove(ctx, inboundPath(tenant, did))
}

// ---- outbound routes

type Pattern struct {
	Prefix  string `json:"prefix"`
	Match   string `json:"match"`
	Prepend string `json:"prepend"`
}

type OutboundRoute struct {
	ID       int64     `json:"id"`
	TenantID int64     `json:"tenant_id"`
	Name     string    `json:"name"`
	Position int       `json:"position"`
	Enabled  bool      `json:"enabled"`
	Patterns []Pattern `json:"patterns"`
	Trunks   []int64   `json:"trunks"`
}

type OutboundRouteInput struct {
	Name     string    `json:"name"`
	Position int       `json:"position"`
	Patterns []Pattern `json:"patterns"`
	Trunks   []int64   `json:"trunks"`
}

type OutboundRoutePatch struct {
	Name     *string    `json:"name,omitempty"`
	Position *int       `json:"position,omitempty"`
	Enabled  *bool      `json:"enabled,omitempty"`
	Patterns *[]Pattern `json:"patterns,omitempty"`
	Trunks   *[]int64   `json:"trunks,omitempty"`
}

func (c *Client) CreateOutboundRoute(ctx context.Context, tenant int64, in OutboundRouteInput) (OutboundRoute, error) {
	return create[OutboundRoute](ctx, c, tp(tenant, "outbound-routes"), in)
}

func (c *Client) GetOutboundRoute(ctx context.Context, tenant, id int64) (OutboundRoute, error) {
	return get[OutboundRoute](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "outbound-routes"), id))
}

func (c *Client) UpdateOutboundRoute(ctx context.Context, tenant, id int64, p OutboundRoutePatch) (OutboundRoute, error) {
	return patch[OutboundRoute](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "outbound-routes"), id), p)
}

func (c *Client) DeleteOutboundRoute(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "outbound-routes"), id))
}
