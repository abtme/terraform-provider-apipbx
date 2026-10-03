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

// doRaw sends body as is (audio) and decodes a JSON answer into out; with wantBytes it returns the raw answer.
func (c *Client) doRaw(ctx context.Context, method, path, contentType string, body []byte, out any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		msg := fmt.Errorf("%s %s: %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %w", ErrNotFound, msg)
		}
		return nil, msg
	}
	if out != nil {
		return b, json.Unmarshal(b, out)
	}
	return b, nil
}

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
	// VoicemailBoxID is the extension's mailbox (nil: none).
	VoicemailBoxID       *int64 `json:"voicemail_box_id"`
	Language             string `json:"language"`
	DND                  bool   `json:"dnd"`
	ForwardUnconditional string `json:"forward_unconditional"`
	ForwardBusy          string `json:"forward_busy"`
	ForwardNoAnswer      string `json:"forward_no_answer"`
	ForwardUnavailable   string `json:"forward_unavailable"`
}

type ExtensionInput struct {
	Number               string `json:"number"`
	Name                 string `json:"name"`
	Tech                 string `json:"tech,omitempty"`
	Secret               string `json:"secret,omitempty"` // generated when empty
	OutboundCID          string `json:"outbound_cid"`
	RingTime             int    `json:"ring_time,omitempty"`
	VoicemailBoxID       *int64 `json:"voicemail_box_id,omitempty"`
	Language             string `json:"language,omitempty"`
	DND                  bool   `json:"dnd,omitempty"`
	ForwardUnconditional string `json:"forward_unconditional,omitempty"`
	ForwardBusy          string `json:"forward_busy,omitempty"`
	ForwardNoAnswer      string `json:"forward_no_answer,omitempty"`
	ForwardUnavailable   string `json:"forward_unavailable,omitempty"`
}

type ExtensionPatch struct {
	Name        *string `json:"name,omitempty"`
	Secret      *string `json:"secret,omitempty"`
	OutboundCID *string `json:"outbound_cid,omitempty"`
	RingTime    *int    `json:"ring_time,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	// VoicemailBoxID links a mailbox; 0 removes the link.
	VoicemailBoxID       *int64  `json:"voicemail_box_id,omitempty"`
	DND                  *bool   `json:"dnd,omitempty"`
	ForwardUnconditional *string `json:"forward_unconditional,omitempty"`
	ForwardBusy          *string `json:"forward_busy,omitempty"`
	ForwardNoAnswer      *string `json:"forward_no_answer,omitempty"`
	ForwardUnavailable   *string `json:"forward_unavailable,omitempty"`
	Language             *string `json:"language,omitempty"`
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
	Language    string      `json:"language"`
}

type InboundRouteInput struct {
	DID         string      `json:"did"`
	Description string      `json:"description"`
	CIDPrefix   string      `json:"cid_prefix"`
	Destination Destination `json:"destination"`
	Language    string      `json:"language"`
}

type InboundRoutePatch struct {
	Description *string      `json:"description,omitempty"`
	CIDPrefix   *string      `json:"cid_prefix,omitempty"`
	Destination *Destination `json:"destination,omitempty"`
	Language    *string      `json:"language,omitempty"`
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
	PinSetID *int64    `json:"pin_set_id"`
}

type OutboundRouteInput struct {
	Name     string    `json:"name"`
	Position int       `json:"position"`
	Patterns []Pattern `json:"patterns"`
	Trunks   []int64   `json:"trunks"`
	PinSetID *int64    `json:"pin_set_id,omitempty"`
}

type OutboundRoutePatch struct {
	Name     *string    `json:"name,omitempty"`
	Position *int       `json:"position,omitempty"`
	Enabled  *bool      `json:"enabled,omitempty"`
	Patterns *[]Pattern `json:"patterns,omitempty"`
	Trunks   *[]int64   `json:"trunks,omitempty"`
	PinSetID *int64     `json:"pin_set_id,omitempty"` // 0 removes the PIN requirement
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

// ---- voicemail boxes

type VoicemailBox struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenant_id"`
	Number           string `json:"number"`
	Name             string `json:"name"`
	PIN              string `json:"pin"`
	Email            string `json:"email"`
	Attach           bool   `json:"attach"`
	DeleteAfterEmail bool   `json:"delete_after_email"`
	SayCID           bool   `json:"say_cid"`
	Envelope         bool   `json:"envelope"`
	MaxMessages      int    `json:"max_messages"`
	Language         string `json:"language"`
}

type VoicemailBoxInput struct {
	Number           string `json:"number"`
	Name             string `json:"name"`
	PIN              string `json:"pin,omitempty"` // generated when empty
	Email            string `json:"email"`
	Attach           bool   `json:"attach"`
	DeleteAfterEmail bool   `json:"delete_after_email"`
	SayCID           bool   `json:"say_cid"`
	Envelope         bool   `json:"envelope"`
	MaxMessages      int    `json:"max_messages,omitempty"`
	Language         string `json:"language,omitempty"`
}

type VoicemailBoxPatch struct {
	Name             *string `json:"name,omitempty"`
	PIN              *string `json:"pin,omitempty"`
	Email            *string `json:"email,omitempty"`
	Attach           *bool   `json:"attach,omitempty"`
	DeleteAfterEmail *bool   `json:"delete_after_email,omitempty"`
	SayCID           *bool   `json:"say_cid,omitempty"`
	Envelope         *bool   `json:"envelope,omitempty"`
	MaxMessages      *int    `json:"max_messages,omitempty"`
	Language         *string `json:"language,omitempty"`
}

func (c *Client) CreateVoicemailBox(ctx context.Context, tenant int64, in VoicemailBoxInput) (VoicemailBox, error) {
	return create[VoicemailBox](ctx, c, tp(tenant, "voicemail-boxes"), in)
}

func (c *Client) GetVoicemailBox(ctx context.Context, tenant, id int64) (VoicemailBox, error) {
	return get[VoicemailBox](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "voicemail-boxes"), id))
}

func (c *Client) UpdateVoicemailBox(ctx context.Context, tenant, id int64, p VoicemailBoxPatch) (VoicemailBox, error) {
	return patch[VoicemailBox](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "voicemail-boxes"), id), p)
}

func (c *Client) DeleteVoicemailBox(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "voicemail-boxes"), id))
}

// ---- time groups and conditions

type TimeRange struct {
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Weekdays  []string `json:"weekdays"`
	MonthDays []int    `json:"month_days"`
	Months    []int    `json:"months"`
}

type TimeGroup struct {
	ID       int64       `json:"id"`
	TenantID int64       `json:"tenant_id"`
	Name     string      `json:"name"`
	Timezone string      `json:"timezone"`
	Ranges   []TimeRange `json:"ranges"`
}

type TimeGroupInput struct {
	Name     string      `json:"name"`
	Timezone string      `json:"timezone"`
	Ranges   []TimeRange `json:"ranges"`
}

type TimeGroupPatch struct {
	Name     *string      `json:"name,omitempty"`
	Timezone *string      `json:"timezone,omitempty"`
	Ranges   *[]TimeRange `json:"ranges,omitempty"`
}

func (c *Client) CreateTimeGroup(ctx context.Context, tenant int64, in TimeGroupInput) (TimeGroup, error) {
	return create[TimeGroup](ctx, c, tp(tenant, "time-groups"), in)
}

func (c *Client) GetTimeGroup(ctx context.Context, tenant, id int64) (TimeGroup, error) {
	return get[TimeGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "time-groups"), id))
}

func (c *Client) UpdateTimeGroup(ctx context.Context, tenant, id int64, p TimeGroupPatch) (TimeGroup, error) {
	return patch[TimeGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "time-groups"), id), p)
}

func (c *Client) DeleteTimeGroup(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "time-groups"), id))
}

type TimeCondition struct {
	ID          int64       `json:"id"`
	TenantID    int64       `json:"tenant_id"`
	Name        string      `json:"name"`
	TimeGroupID int64       `json:"time_group_id"`
	Match       Destination `json:"match"`
	NoMatch     Destination `json:"no_match"`
	Override    string      `json:"override"`
	ToggleCode  string      `json:"toggle_code"`
	MatchesNow  bool        `json:"matches_now"`
}

type TimeConditionInput struct {
	Name        string      `json:"name"`
	TimeGroupID int64       `json:"time_group_id"`
	Match       Destination `json:"match"`
	NoMatch     Destination `json:"no_match"`
	Override    string      `json:"override,omitempty"`
	ToggleCode  string      `json:"toggle_code,omitempty"`
}

type TimeConditionPatch struct {
	Name        *string      `json:"name,omitempty"`
	TimeGroupID *int64       `json:"time_group_id,omitempty"`
	Match       *Destination `json:"match,omitempty"`
	NoMatch     *Destination `json:"no_match,omitempty"`
	Override    *string      `json:"override,omitempty"`
	ToggleCode  *string      `json:"toggle_code,omitempty"`
}

func (c *Client) CreateTimeCondition(ctx context.Context, tenant int64, in TimeConditionInput) (TimeCondition, error) {
	return create[TimeCondition](ctx, c, tp(tenant, "time-conditions"), in)
}

func (c *Client) GetTimeCondition(ctx context.Context, tenant, id int64) (TimeCondition, error) {
	return get[TimeCondition](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "time-conditions"), id))
}

func (c *Client) UpdateTimeCondition(ctx context.Context, tenant, id int64, p TimeConditionPatch) (TimeCondition, error) {
	return patch[TimeCondition](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "time-conditions"), id), p)
}

func (c *Client) DeleteTimeCondition(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "time-conditions"), id))
}

// ---- IVRs

type IVREntry struct {
	Digit       string      `json:"digit"`
	Destination Destination `json:"destination"`
}

type IVR struct {
	ID                 int64       `json:"id"`
	TenantID           int64       `json:"tenant_id"`
	Name               string      `json:"name"`
	Announcement       string      `json:"announcement"`
	RecordingID        *int64      `json:"recording_id"`
	TimeoutSeconds     int         `json:"timeout_seconds"`
	MaxRetries         int         `json:"max_retries"`
	Entries            []IVREntry  `json:"entries"`
	TimeoutDestination Destination `json:"timeout_destination"`
	InvalidDestination Destination `json:"invalid_destination"`
}

type IVRInput struct {
	Name               string       `json:"name"`
	Announcement       string       `json:"announcement"`
	RecordingID        *int64       `json:"recording_id,omitempty"`
	TimeoutSeconds     int          `json:"timeout_seconds,omitempty"`
	MaxRetries         int          `json:"max_retries,omitempty"`
	Entries            []IVREntry   `json:"entries"`
	TimeoutDestination *Destination `json:"timeout_destination,omitempty"`
	InvalidDestination *Destination `json:"invalid_destination,omitempty"`
}

type IVRPatch struct {
	Name               *string      `json:"name,omitempty"`
	Announcement       *string      `json:"announcement,omitempty"`
	RecordingID        *int64       `json:"recording_id,omitempty"` // 0 removes the recording
	TimeoutSeconds     *int         `json:"timeout_seconds,omitempty"`
	MaxRetries         *int         `json:"max_retries,omitempty"`
	Entries            *[]IVREntry  `json:"entries,omitempty"`
	TimeoutDestination *Destination `json:"timeout_destination,omitempty"`
	InvalidDestination *Destination `json:"invalid_destination,omitempty"`
}

func (c *Client) CreateIVR(ctx context.Context, tenant int64, in IVRInput) (IVR, error) {
	return create[IVR](ctx, c, tp(tenant, "ivrs"), in)
}

func (c *Client) GetIVR(ctx context.Context, tenant, id int64) (IVR, error) {
	return get[IVR](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "ivrs"), id))
}

func (c *Client) UpdateIVR(ctx context.Context, tenant, id int64, p IVRPatch) (IVR, error) {
	return patch[IVR](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "ivrs"), id), p)
}

func (c *Client) DeleteIVR(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "ivrs"), id))
}

// ---- recordings

type Recording struct {
	ID              int64   `json:"id"`
	TenantID        int64   `json:"tenant_id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	DurationSeconds float64 `json:"duration_seconds"`
	SizeBytes       int     `json:"size_bytes"`
	SHA256          string  `json:"sha256"`
}

type RecordingInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Audio       []byte `json:"audio"` // base64 in JSON
}

type RecordingPatch struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Audio       *[]byte `json:"audio,omitempty"`
}

func (c *Client) CreateRecording(ctx context.Context, tenant int64, in RecordingInput) (Recording, error) {
	return create[Recording](ctx, c, tp(tenant, "recordings"), in)
}

func (c *Client) GetRecording(ctx context.Context, tenant, id int64) (Recording, error) {
	return get[Recording](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "recordings"), id))
}

func (c *Client) UpdateRecording(ctx context.Context, tenant, id int64, p RecordingPatch) (Recording, error) {
	return patch[Recording](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "recordings"), id), p)
}

func (c *Client) DeleteRecording(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "recordings"), id))
}

// ---- voicemail greetings

type VoicemailGreeting struct {
	Type            string  `json:"type"`
	DurationSeconds float64 `json:"duration_seconds"`
	SizeBytes       int     `json:"size_bytes"`
}

func greetingPath(tenant, box int64, kind string) string {
	return fmt.Sprintf("%s/%d/greetings/%s", tp(tenant, "voicemail-boxes"), box, kind)
}

func (c *Client) SetVoicemailGreeting(ctx context.Context, tenant, box int64, kind string, wav []byte) (VoicemailGreeting, error) {
	var g VoicemailGreeting
	_, err := c.doRaw(ctx, "PUT", greetingPath(tenant, box, kind), "audio/wav", wav, &g)
	return g, err
}

func (c *Client) GetVoicemailGreetingAudio(ctx context.Context, tenant, box int64, kind string) ([]byte, error) {
	return c.doRaw(ctx, "GET", greetingPath(tenant, box, kind), "", nil, nil)
}

func (c *Client) GetVoicemailGreeting(ctx context.Context, tenant, box int64, kind string) (VoicemailGreeting, error) {
	return get[VoicemailGreeting](ctx, c, greetingPath(tenant, box, kind)+"/info")
}

func (c *Client) DeleteVoicemailGreeting(ctx context.Context, tenant, box int64, kind string) error {
	return c.remove(ctx, greetingPath(tenant, box, kind))
}

// ---- blacklist

type BlacklistEntry struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	Number      string `json:"number"`
	Description string `json:"description"`
}

type BlacklistEntryInput struct {
	Number      string `json:"number"`
	Description string `json:"description"`
}

type BlacklistEntryPatch struct {
	Description *string `json:"description,omitempty"`
}

func (c *Client) CreateBlacklistEntry(ctx context.Context, tenant int64, in BlacklistEntryInput) (BlacklistEntry, error) {
	return create[BlacklistEntry](ctx, c, tp(tenant, "blacklist"), in)
}

func (c *Client) GetBlacklistEntry(ctx context.Context, tenant, id int64) (BlacklistEntry, error) {
	return get[BlacklistEntry](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "blacklist"), id))
}

func (c *Client) UpdateBlacklistEntry(ctx context.Context, tenant, id int64, p BlacklistEntryPatch) (BlacklistEntry, error) {
	return patch[BlacklistEntry](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "blacklist"), id), p)
}

func (c *Client) DeleteBlacklistEntry(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "blacklist"), id))
}

type BlacklistSettings struct {
	BlockUnknown bool        `json:"block_unknown"`
	Destination  Destination `json:"destination"`
}

func (c *Client) GetBlacklistSettings(ctx context.Context, tenant int64) (BlacklistSettings, error) {
	return get[BlacklistSettings](ctx, c, tp(tenant, "blacklist-settings"))
}

func (c *Client) UpdateBlacklistSettings(ctx context.Context, tenant int64, s BlacklistSettings) (BlacklistSettings, error) {
	return patch[BlacklistSettings](ctx, c, tp(tenant, "blacklist-settings"), s)
}

// ---- queues

type QueueMember struct {
	ExtensionID int64 `json:"extension_id"`
	Penalty     int   `json:"penalty"`
	Dynamic     bool  `json:"dynamic"`
}

type CallQueue struct {
	ID                 int64         `json:"id"`
	TenantID           int64         `json:"tenant_id"`
	Number             string        `json:"number"`
	Name               string        `json:"name"`
	Strategy           string        `json:"strategy"`
	AgentTimeout       int           `json:"agent_timeout"`
	Retry              int           `json:"retry"`
	WrapupTime         int           `json:"wrapup_time"`
	MaxWait            int           `json:"max_wait"`
	MaxCallers         int           `json:"max_callers"`
	JoinEmpty          bool          `json:"join_empty"`
	LeaveWhenEmpty     bool          `json:"leave_when_empty"`
	AnnouncePosition   bool          `json:"announce_position"`
	AnnounceHoldTime   bool          `json:"announce_hold_time"`
	AnnounceFrequency  int           `json:"announce_frequency"`
	MusicOnHold        string        `json:"music_on_hold"`
	Members            []QueueMember `json:"members"`
	TimeoutDestination Destination   `json:"timeout_destination"`
}

type CallQueueInput struct {
	Number             string        `json:"number"`
	Name               string        `json:"name"`
	Strategy           string        `json:"strategy,omitempty"`
	AgentTimeout       int           `json:"agent_timeout,omitempty"`
	Retry              *int          `json:"retry,omitempty"`
	WrapupTime         int           `json:"wrapup_time,omitempty"`
	MaxWait            int           `json:"max_wait,omitempty"`
	MaxCallers         int           `json:"max_callers,omitempty"`
	JoinEmpty          bool          `json:"join_empty,omitempty"`
	LeaveWhenEmpty     bool          `json:"leave_when_empty,omitempty"`
	AnnouncePosition   bool          `json:"announce_position,omitempty"`
	AnnounceHoldTime   bool          `json:"announce_hold_time,omitempty"`
	AnnounceFrequency  int           `json:"announce_frequency,omitempty"`
	MusicOnHold        string        `json:"music_on_hold,omitempty"`
	Members            []QueueMember `json:"members"`
	TimeoutDestination *Destination  `json:"timeout_destination,omitempty"`
}

type CallQueuePatch struct {
	Name               *string        `json:"name,omitempty"`
	Strategy           *string        `json:"strategy,omitempty"`
	AgentTimeout       *int           `json:"agent_timeout,omitempty"`
	Retry              *int           `json:"retry,omitempty"`
	WrapupTime         *int           `json:"wrapup_time,omitempty"`
	MaxWait            *int           `json:"max_wait,omitempty"`
	MaxCallers         *int           `json:"max_callers,omitempty"`
	JoinEmpty          *bool          `json:"join_empty,omitempty"`
	LeaveWhenEmpty     *bool          `json:"leave_when_empty,omitempty"`
	AnnouncePosition   *bool          `json:"announce_position,omitempty"`
	AnnounceHoldTime   *bool          `json:"announce_hold_time,omitempty"`
	AnnounceFrequency  *int           `json:"announce_frequency,omitempty"`
	MusicOnHold        *string        `json:"music_on_hold,omitempty"`
	Members            *[]QueueMember `json:"members,omitempty"`
	TimeoutDestination *Destination   `json:"timeout_destination,omitempty"`
}

func (c *Client) CreateCallQueue(ctx context.Context, tenant int64, in CallQueueInput) (CallQueue, error) {
	return create[CallQueue](ctx, c, tp(tenant, "queues"), in)
}

func (c *Client) GetCallQueue(ctx context.Context, tenant, id int64) (CallQueue, error) {
	return get[CallQueue](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "queues"), id))
}

func (c *Client) UpdateCallQueue(ctx context.Context, tenant, id int64, p CallQueuePatch) (CallQueue, error) {
	return patch[CallQueue](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "queues"), id), p)
}

func (c *Client) DeleteCallQueue(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "queues"), id))
}

// ---- conferences

type Conference struct {
	ID         int64  `json:"id"`
	TenantID   int64  `json:"tenant_id"`
	Number     string `json:"number"`
	Name       string `json:"name"`
	PIN        string `json:"pin"`
	AdminPIN   string `json:"admin_pin"`
	MaxMembers int    `json:"max_members"`
	MuteOnJoin bool   `json:"mute_on_join"`
}

type ConferenceInput struct {
	Number     string `json:"number"`
	Name       string `json:"name"`
	PIN        string `json:"pin"`
	AdminPIN   string `json:"admin_pin"`
	MaxMembers int    `json:"max_members"`
	MuteOnJoin bool   `json:"mute_on_join"`
}

type ConferencePatch struct {
	Name       *string `json:"name,omitempty"`
	PIN        *string `json:"pin,omitempty"`
	AdminPIN   *string `json:"admin_pin,omitempty"`
	MaxMembers *int    `json:"max_members,omitempty"`
	MuteOnJoin *bool   `json:"mute_on_join,omitempty"`
}

func (c *Client) CreateConference(ctx context.Context, tenant int64, in ConferenceInput) (Conference, error) {
	return create[Conference](ctx, c, tp(tenant, "conferences"), in)
}

func (c *Client) GetConference(ctx context.Context, tenant, id int64) (Conference, error) {
	return get[Conference](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "conferences"), id))
}

func (c *Client) UpdateConference(ctx context.Context, tenant, id int64, p ConferencePatch) (Conference, error) {
	return patch[Conference](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "conferences"), id), p)
}

func (c *Client) DeleteConference(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "conferences"), id))
}

// ---- announcements, misc destinations, caller id steps, speed dials

type Announcement struct {
	ID          int64       `json:"id"`
	TenantID    int64       `json:"tenant_id"`
	Name        string      `json:"name"`
	RecordingID int64       `json:"recording_id"`
	Next        Destination `json:"next"`
}

type AnnouncementInput struct {
	Name        string       `json:"name"`
	RecordingID int64        `json:"recording_id"`
	Next        *Destination `json:"next,omitempty"`
}

type AnnouncementPatch struct {
	Name        *string      `json:"name,omitempty"`
	RecordingID *int64       `json:"recording_id,omitempty"`
	Next        *Destination `json:"next,omitempty"`
}

func (c *Client) CreateAnnouncement(ctx context.Context, tenant int64, in AnnouncementInput) (Announcement, error) {
	return create[Announcement](ctx, c, tp(tenant, "announcements"), in)
}
func (c *Client) GetAnnouncement(ctx context.Context, tenant, id int64) (Announcement, error) {
	return get[Announcement](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "announcements"), id))
}
func (c *Client) UpdateAnnouncement(ctx context.Context, tenant, id int64, p AnnouncementPatch) (Announcement, error) {
	return patch[Announcement](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "announcements"), id), p)
}
func (c *Client) DeleteAnnouncement(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "announcements"), id))
}

type MiscDestination struct {
	ID       int64  `json:"id"`
	TenantID int64  `json:"tenant_id"`
	Name     string `json:"name"`
	Number   string `json:"number"`
}

type MiscDestinationInput struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

type MiscDestinationPatch struct {
	Name   *string `json:"name,omitempty"`
	Number *string `json:"number,omitempty"`
}

func (c *Client) CreateMiscDestination(ctx context.Context, tenant int64, in MiscDestinationInput) (MiscDestination, error) {
	return create[MiscDestination](ctx, c, tp(tenant, "misc-destinations"), in)
}
func (c *Client) GetMiscDestination(ctx context.Context, tenant, id int64) (MiscDestination, error) {
	return get[MiscDestination](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "misc-destinations"), id))
}
func (c *Client) UpdateMiscDestination(ctx context.Context, tenant, id int64, p MiscDestinationPatch) (MiscDestination, error) {
	return patch[MiscDestination](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "misc-destinations"), id), p)
}
func (c *Client) DeleteMiscDestination(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "misc-destinations"), id))
}

type CallerIDStep struct {
	ID         int64       `json:"id"`
	TenantID   int64       `json:"tenant_id"`
	Name       string      `json:"name"`
	NamePrefix string      `json:"name_prefix"`
	Number     string      `json:"number"`
	Next       Destination `json:"next"`
}

type CallerIDStepInput struct {
	Name       string       `json:"name"`
	NamePrefix string       `json:"name_prefix"`
	Number     string       `json:"number"`
	Next       *Destination `json:"next,omitempty"`
}

type CallerIDStepPatch struct {
	Name       *string      `json:"name,omitempty"`
	NamePrefix *string      `json:"name_prefix,omitempty"`
	Number     *string      `json:"number,omitempty"`
	Next       *Destination `json:"next,omitempty"`
}

func (c *Client) CreateCallerIDStep(ctx context.Context, tenant int64, in CallerIDStepInput) (CallerIDStep, error) {
	return create[CallerIDStep](ctx, c, tp(tenant, "caller-id-steps"), in)
}
func (c *Client) GetCallerIDStep(ctx context.Context, tenant, id int64) (CallerIDStep, error) {
	return get[CallerIDStep](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "caller-id-steps"), id))
}
func (c *Client) UpdateCallerIDStep(ctx context.Context, tenant, id int64, p CallerIDStepPatch) (CallerIDStep, error) {
	return patch[CallerIDStep](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "caller-id-steps"), id), p)
}
func (c *Client) DeleteCallerIDStep(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "caller-id-steps"), id))
}

type SpeedDial struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	ExtensionID *int64 `json:"extension_id"`
	Code        string `json:"code"`
	Number      string `json:"number"`
	Description string `json:"description"`
}

type SpeedDialInput struct {
	ExtensionID *int64 `json:"extension_id,omitempty"`
	Code        string `json:"code"`
	Number      string `json:"number"`
	Description string `json:"description"`
}

type SpeedDialPatch struct {
	Number      *string `json:"number,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (c *Client) CreateSpeedDial(ctx context.Context, tenant int64, in SpeedDialInput) (SpeedDial, error) {
	return create[SpeedDial](ctx, c, tp(tenant, "speed-dials"), in)
}
func (c *Client) GetSpeedDial(ctx context.Context, tenant, id int64) (SpeedDial, error) {
	return get[SpeedDial](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "speed-dials"), id))
}
func (c *Client) UpdateSpeedDial(ctx context.Context, tenant, id int64, p SpeedDialPatch) (SpeedDial, error) {
	return patch[SpeedDial](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "speed-dials"), id), p)
}
func (c *Client) DeleteSpeedDial(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "speed-dials"), id))
}

// ---- paging groups

type PagingGroup struct {
	ID       int64   `json:"id"`
	TenantID int64   `json:"tenant_id"`
	Number   string  `json:"number"`
	Name     string  `json:"name"`
	Duplex   bool    `json:"duplex"`
	Members  []int64 `json:"members"`
}

type PagingGroupInput struct {
	Number  string  `json:"number"`
	Name    string  `json:"name"`
	Duplex  bool    `json:"duplex"`
	Members []int64 `json:"members"`
}

type PagingGroupPatch struct {
	Name    *string  `json:"name,omitempty"`
	Duplex  *bool    `json:"duplex,omitempty"`
	Members *[]int64 `json:"members,omitempty"`
}

func (c *Client) CreatePagingGroup(ctx context.Context, tenant int64, in PagingGroupInput) (PagingGroup, error) {
	return create[PagingGroup](ctx, c, tp(tenant, "paging-groups"), in)
}
func (c *Client) GetPagingGroup(ctx context.Context, tenant, id int64) (PagingGroup, error) {
	return get[PagingGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "paging-groups"), id))
}
func (c *Client) UpdatePagingGroup(ctx context.Context, tenant, id int64, p PagingGroupPatch) (PagingGroup, error) {
	return patch[PagingGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "paging-groups"), id), p)
}
func (c *Client) DeletePagingGroup(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "paging-groups"), id))
}

// ---- voicemail groups

type VoicemailGroup struct {
	ID       int64   `json:"id"`
	TenantID int64   `json:"tenant_id"`
	Number   string  `json:"number"`
	Name     string  `json:"name"`
	Boxes    []int64 `json:"boxes"`
}

type VoicemailGroupInput struct {
	Number string  `json:"number"`
	Name   string  `json:"name"`
	Boxes  []int64 `json:"boxes"`
}

type VoicemailGroupPatch struct {
	Name  *string  `json:"name,omitempty"`
	Boxes *[]int64 `json:"boxes,omitempty"`
}

func (c *Client) CreateVoicemailGroup(ctx context.Context, tenant int64, in VoicemailGroupInput) (VoicemailGroup, error) {
	return create[VoicemailGroup](ctx, c, tp(tenant, "voicemail-groups"), in)
}
func (c *Client) GetVoicemailGroup(ctx context.Context, tenant, id int64) (VoicemailGroup, error) {
	return get[VoicemailGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "voicemail-groups"), id))
}
func (c *Client) UpdateVoicemailGroup(ctx context.Context, tenant, id int64, p VoicemailGroupPatch) (VoicemailGroup, error) {
	return patch[VoicemailGroup](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "voicemail-groups"), id), p)
}
func (c *Client) DeleteVoicemailGroup(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "voicemail-groups"), id))
}

// ---- follow-me

type FollowMe struct {
	ExtensionID    int64    `json:"extension_id"`
	Enabled        bool     `json:"enabled"`
	Numbers        []string `json:"numbers"`
	PreringSeconds int      `json:"prering_seconds"`
	RingSeconds    int      `json:"ring_seconds"`
	Confirm        bool     `json:"confirm"`
}

type FollowMeInput struct {
	Enabled        bool     `json:"enabled"`
	Numbers        []string `json:"numbers"`
	PreringSeconds int      `json:"prering_seconds"`
	RingSeconds    int      `json:"ring_seconds"`
	Confirm        bool     `json:"confirm"`
}

func followMePath(tenant, ext int64) string {
	return fmt.Sprintf("%s/%d/follow-me", tp(tenant, "extensions"), ext)
}

func (c *Client) SetFollowMe(ctx context.Context, tenant, ext int64, in FollowMeInput) (FollowMe, error) {
	var f FollowMe
	err := c.do(ctx, "PUT", followMePath(tenant, ext), in, &f)
	return f, err
}

func (c *Client) GetFollowMe(ctx context.Context, tenant, ext int64) (FollowMe, error) {
	return get[FollowMe](ctx, c, followMePath(tenant, ext))
}

func (c *Client) DeleteFollowMe(ctx context.Context, tenant, ext int64) error {
	return c.remove(ctx, followMePath(tenant, ext))
}

// ---- music on hold classes

type MohClass struct {
	ID         int64   `json:"id"`
	TenantID   int64   `json:"tenant_id"`
	Name       string  `json:"name"`
	Random     bool    `json:"random"`
	Recordings []int64 `json:"recordings"`
}

type MohClassInput struct {
	Name       string  `json:"name"`
	Random     bool    `json:"random"`
	Recordings []int64 `json:"recordings"`
}

type MohClassPatch struct {
	Random     *bool    `json:"random,omitempty"`
	Recordings *[]int64 `json:"recordings,omitempty"`
}

func (c *Client) CreateMohClass(ctx context.Context, tenant int64, in MohClassInput) (MohClass, error) {
	return create[MohClass](ctx, c, tp(tenant, "moh-classes"), in)
}
func (c *Client) GetMohClass(ctx context.Context, tenant, id int64) (MohClass, error) {
	return get[MohClass](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "moh-classes"), id))
}
func (c *Client) UpdateMohClass(ctx context.Context, tenant, id int64, p MohClassPatch) (MohClass, error) {
	return patch[MohClass](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "moh-classes"), id), p)
}
func (c *Client) DeleteMohClass(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "moh-classes"), id))
}

// ---- call recording policy

type CallRecordingPolicy struct {
	ExtensionID int64  `json:"extension_id"`
	Mode        string `json:"mode"`
}

func (c *Client) SetCallRecordingPolicy(ctx context.Context, tenant, ext int64, mode string) (CallRecordingPolicy, error) {
	var p CallRecordingPolicy
	err := c.do(ctx, "PUT", fmt.Sprintf("%s/%d/call-recording", tp(tenant, "extensions"), ext), map[string]string{"mode": mode}, &p)
	return p, err
}

func (c *Client) GetCallRecordingPolicy(ctx context.Context, tenant, ext int64) (CallRecordingPolicy, error) {
	return get[CallRecordingPolicy](ctx, c, fmt.Sprintf("%s/%d/call-recording", tp(tenant, "extensions"), ext))
}

// ---- parking lot

type ParkingLot struct {
	ParkNumber string `json:"park_number"`
	FirstSlot  int    `json:"first_slot"`
	Slots      int    `json:"slots"`
}

func (c *Client) SetParkingLot(ctx context.Context, tenant int64, in ParkingLot) (ParkingLot, error) {
	var l ParkingLot
	err := c.do(ctx, "PUT", fmt.Sprintf("/v1/tenants/%d/parking", tenant), in, &l)
	return l, err
}

func (c *Client) GetParkingLot(ctx context.Context, tenant int64) (ParkingLot, error) {
	return get[ParkingLot](ctx, c, fmt.Sprintf("/v1/tenants/%d/parking", tenant))
}

func (c *Client) DeleteParkingLot(ctx context.Context, tenant int64) error {
	return c.remove(ctx, fmt.Sprintf("/v1/tenants/%d/parking", tenant))
}

// ---- call limit

type CallLimit struct {
	MaxCalls int `json:"max_calls"`
}

func (c *Client) SetCallLimit(ctx context.Context, tenant int64, in CallLimit) (CallLimit, error) {
	var l CallLimit
	err := c.do(ctx, "PUT", fmt.Sprintf("/v1/tenants/%d/call-limit", tenant), in, &l)
	return l, err
}

func (c *Client) GetCallLimit(ctx context.Context, tenant int64) (CallLimit, error) {
	return get[CallLimit](ctx, c, fmt.Sprintf("/v1/tenants/%d/call-limit", tenant))
}

func (c *Client) DeleteCallLimit(ctx context.Context, tenant int64) error {
	return c.remove(ctx, fmt.Sprintf("/v1/tenants/%d/call-limit", tenant))
}

// ---- PIN sets

type PinSet struct {
	ID       int64    `json:"id"`
	TenantID int64    `json:"tenant_id"`
	Name     string   `json:"name"`
	Pins     []string `json:"pins"`
}

type PinSetInput struct {
	Name string   `json:"name"`
	Pins []string `json:"pins"`
}

type PinSetPatch struct {
	Name *string   `json:"name,omitempty"`
	Pins *[]string `json:"pins,omitempty"`
}

func (c *Client) CreatePinSet(ctx context.Context, tenant int64, in PinSetInput) (PinSet, error) {
	return create[PinSet](ctx, c, tp(tenant, "pin-sets"), in)
}
func (c *Client) GetPinSet(ctx context.Context, tenant, id int64) (PinSet, error) {
	return get[PinSet](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "pin-sets"), id))
}
func (c *Client) UpdatePinSet(ctx context.Context, tenant, id int64, p PinSetPatch) (PinSet, error) {
	return patch[PinSet](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "pin-sets"), id), p)
}
func (c *Client) DeletePinSet(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "pin-sets"), id))
}

// ---- DISA

type DISA struct {
	ID       int64  `json:"id"`
	TenantID int64  `json:"tenant_id"`
	Name     string `json:"name"`
	PinSetID int64  `json:"pin_set_id"`
	CallerID string `json:"caller_id"`
}

type DISAInput struct {
	Name     string `json:"name"`
	PinSetID int64  `json:"pin_set_id"`
	CallerID string `json:"caller_id"`
}

type DISAPatch struct {
	Name     *string `json:"name,omitempty"`
	PinSetID *int64  `json:"pin_set_id,omitempty"`
	CallerID *string `json:"caller_id,omitempty"`
}

func (c *Client) CreateDISA(ctx context.Context, tenant int64, in DISAInput) (DISA, error) {
	return create[DISA](ctx, c, tp(tenant, "disas"), in)
}
func (c *Client) GetDISA(ctx context.Context, tenant, id int64) (DISA, error) {
	return get[DISA](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "disas"), id))
}
func (c *Client) UpdateDISA(ctx context.Context, tenant, id int64, p DISAPatch) (DISA, error) {
	return patch[DISA](ctx, c, fmt.Sprintf("%s/%d", tp(tenant, "disas"), id), p)
}
func (c *Client) DeleteDISA(ctx context.Context, tenant, id int64) error {
	return c.remove(ctx, fmt.Sprintf("%s/%d", tp(tenant, "disas"), id))
}
