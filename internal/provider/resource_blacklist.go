package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

// ---- apipbx_blacklist_entry

type blacklistEntryResource struct{ c *client.Client }

type blacklistEntryModel struct {
	ID          types.String `tfsdk:"id"`
	TenantID    types.String `tfsdk:"tenant_id"`
	Number      types.String `tfsdk:"number"`
	Description types.String `tfsdk:"description"`
}

func NewBlacklistEntryResource() resource.Resource { return &blacklistEntryResource{} }

func (r *blacklistEntryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_blacklist_entry"
}

func (r *blacklistEntryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A blacklisted caller number. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"number": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "Digits (3-20); a leading + is dropped. Matched exactly against the caller number of calls from trunks."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
		}}
}

func (r *blacklistEntryResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *blacklistEntryModel) set(e client.BlacklistEntry) {
	m.ID, m.TenantID = idString(e.ID), idString(e.TenantID)
	m.Number, m.Description = types.StringValue(e.Number), types.StringValue(e.Description)
}

func (r *blacklistEntryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m blacklistEntryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.c.CreateBlacklistEntry(ctx, tenant, client.BlacklistEntryInput{Number: m.Number.ValueString(), Description: m.Description.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create blacklist entry", err.Error())
		return
	}
	// the API drops a leading +: keep what the user wrote when it means the same number
	if "+"+e.Number != m.Number.ValueString() {
		m.Number = types.StringValue(e.Number)
	}
	m.ID, m.TenantID = idString(e.ID), idString(e.TenantID)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *blacklistEntryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m blacklistEntryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.c.GetBlacklistEntry(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "blacklist entry", err, resp)
		return
	}
	keep := m.Number
	m.set(e)
	if "+"+e.Number == keep.ValueString() {
		m.Number = keep
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *blacklistEntryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m blacklistEntryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.c.UpdateBlacklistEntry(ctx, tenant, id, client.BlacklistEntryPatch{Description: ptr(m.Description.ValueString())})
	if err != nil {
		resp.Diagnostics.AddError("update blacklist entry", err.Error())
		return
	}
	m.Description = types.StringValue(e.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *blacklistEntryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m blacklistEntryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("blacklist entry", r.c.DeleteBlacklistEntry(ctx, tenant, id), resp)
}

func (r *blacklistEntryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}

// ---- apipbx_blacklist_settings (one per tenant)

type blacklistSettingsResource struct{ c *client.Client }

type blacklistSettingsModel struct {
	ID              types.String `tfsdk:"id"`
	TenantID        types.String `tfsdk:"tenant_id"`
	BlockUnknown    types.Bool   `tfsdk:"block_unknown"`
	DestinationType types.String `tfsdk:"destination_type"`
	DestinationID   types.String `tfsdk:"destination_id"`
}

func NewBlacklistSettingsResource() resource.Resource { return &blacklistSettingsResource{} }

func (r *blacklistSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_blacklist_settings"
}

func (r *blacklistSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A tenant's blacklist settings (one per tenant). Destroying it restores the defaults. Import with the tenant id.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"block_unknown": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Also divert calls whose caller id is withheld."},
			"destination_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("hangup"),
				Description: "Where diverted calls go: extension, ring_group, voicemail, time_condition, ivr, queue or hangup."},
			"destination_id": schema.StringAttribute{Optional: true},
		}}
}

func (r *blacklistSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *blacklistSettingsModel) set(tenant int64, s client.BlacklistSettings) {
	m.ID, m.TenantID = idString(tenant), idString(tenant)
	m.BlockUnknown = types.BoolValue(s.BlockUnknown)
	m.DestinationType, m.DestinationID = destination(s.Destination)
}

func (r *blacklistSettingsResource) apply(ctx context.Context, m *blacklistSettingsModel, diags interface {
	AddError(string, string)
}, tenant int64, dest *client.Destination) bool {
	s, err := r.c.UpdateBlacklistSettings(ctx, tenant, client.BlacklistSettings{BlockUnknown: m.BlockUnknown.ValueBool(), Destination: *dest})
	if err != nil {
		diags.AddError("update blacklist settings", err.Error())
		return false
	}
	m.set(tenant, s)
	return true
}

func (r *blacklistSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m blacklistSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	dest := toDestination(&resp.Diagnostics, m.DestinationType, m.DestinationID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, &resp.Diagnostics, tenant, dest) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *blacklistSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m blacklistSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	dest := toDestination(&resp.Diagnostics, m.DestinationType, m.DestinationID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, &resp.Diagnostics, tenant, dest) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *blacklistSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m blacklistSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.c.GetBlacklistSettings(ctx, tenant)
	if err != nil {
		readFailed(ctx, "blacklist settings", err, resp)
		return
	}
	m.set(tenant, s)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *blacklistSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m blacklistSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.c.UpdateBlacklistSettings(ctx, tenant, client.BlacklistSettings{Destination: client.Destination{Type: "hangup"}}); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("reset blacklist settings", err.Error())
	}
}

func (r *blacklistSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
