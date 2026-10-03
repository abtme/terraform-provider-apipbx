package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type followMeResource struct{ c *client.Client }

type followMeModel struct {
	ID             types.String `tfsdk:"id"`
	TenantID       types.String `tfsdk:"tenant_id"`
	ExtensionID    types.String `tfsdk:"extension_id"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Numbers        types.List   `tfsdk:"numbers"`
	PreringSeconds types.Int64  `tfsdk:"prering_seconds"`
	RingSeconds    types.Int64  `tfsdk:"ring_seconds"`
	Confirm        types.Bool   `tfsdk:"confirm"`
}

func NewFollowMeResource() resource.Resource { return &followMeResource{} }

func (r *followMeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_follow_me"
}

func (r *followMeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An extension's follow-me. Import with \"<tenant id>/<extension id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"extension_id": replaceAttr("The extension that follows."),
			"enabled":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"numbers":      schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "1-5 numbers: internal, or external through the outbound routes."},
			"prering_seconds": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(7),
				Description: "The extension's own phone rings alone this long first (0-60)."},
			"ring_seconds": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(20),
				Description: "Then the phone and the numbers ring together this long (5-300)."},
			"confirm": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "An answerer on one of the numbers must press 1."},
		}}
}

func (r *followMeResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *followMeModel) set(tenant int64, f client.FollowMe) {
	m.ID = types.StringValue(fmt.Sprintf("%d/%d", tenant, f.ExtensionID))
	m.TenantID, m.ExtensionID = idString(tenant), idString(f.ExtensionID)
	m.Enabled, m.Confirm = types.BoolValue(f.Enabled), types.BoolValue(f.Confirm)
	m.PreringSeconds, m.RingSeconds = types.Int64Value(int64(f.PreringSeconds)), types.Int64Value(int64(f.RingSeconds))
	m.Numbers = stringsValue(f.Numbers)
}

func (r *followMeResource) apply(ctx context.Context, m *followMeModel, resp interface {
	AddError(string, string)
}, tenant, ext int64) bool {
	var nums []string
	for _, e := range m.Numbers.Elements() {
		nums = append(nums, e.(types.String).ValueString())
	}
	f, err := r.c.SetFollowMe(ctx, tenant, ext, client.FollowMeInput{Enabled: m.Enabled.ValueBool(), Numbers: nums,
		PreringSeconds: int(m.PreringSeconds.ValueInt64()), RingSeconds: int(m.RingSeconds.ValueInt64()), Confirm: m.Confirm.ValueBool()})
	if err != nil {
		resp.AddError("set follow-me", err.Error())
		return false
	}
	m.set(tenant, f)
	return true
}

func (r *followMeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m followMeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, &resp.Diagnostics, tenant, ext) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *followMeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m followMeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, &resp.Diagnostics, tenant, ext) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *followMeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m followMeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	f, err := r.c.GetFollowMe(ctx, tenant, ext)
	if err != nil {
		readFailed(ctx, "follow-me", err, resp)
		return
	}
	m.set(tenant, f)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *followMeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m followMeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("follow-me", r.c.DeleteFollowMe(ctx, tenant, ext), resp)
}

func (r *followMeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tenant, ext, ok := strings.Cut(req.ID, "/")
	if !ok || tenant == "" || ext == "" {
		resp.Diagnostics.AddError("invalid import id", `expected "<tenant id>/<extension id>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), tenant)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("extension_id"), ext)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
