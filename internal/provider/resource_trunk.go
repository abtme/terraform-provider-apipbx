package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type trunkResource struct{ c *client.Client }

type trunkModel struct {
	ID             types.String `tfsdk:"id"`
	TenantID       types.String `tfsdk:"tenant_id"`
	Name           types.String `tfsdk:"name"`
	Tech           types.String `tfsdk:"tech"`
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	Secret         types.String `tfsdk:"secret"`
	Register       types.Bool   `tfsdk:"register"`
	OutboundCID    types.String `tfsdk:"outbound_cid"`
	MaxChannels    types.Int64  `tfsdk:"max_channels"`
	ContinueOnFail types.Bool   `tfsdk:"continue_on_fail"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	AsteriskID     types.String `tfsdk:"asterisk_id"`
}

func NewTrunkResource() resource.Resource { return &trunkResource{} }

func (r *trunkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trunk"
}

func (r *trunkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A trunk to a carrier (SIP or IAX2). Import with \"<tenant id>/<trunk id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"name":      schema.StringAttribute{Required: true, Description: "Starts with a letter; a-z 0-9 _ -.", PlanModifiers: replace},
			"tech":      schema.StringAttribute{Required: true, Description: "pjsip or iax2.", PlanModifiers: replace},
			"host":      schema.StringAttribute{Required: true},
			"port": schema.Int64Attribute{Optional: true, Computed: true,
				Description:   "Provider port; the technology's default when omitted. Cannot be cleared once set.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"username":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"secret":           schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, Default: stringdefault.StaticString("")},
			"register":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Register to the provider (pjsip only; needs username)."},
			"outbound_cid":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"max_channels":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "0 = unlimited."},
			"continue_on_fail": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Try the next trunk after any failure."},
			"enabled":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"asterisk_id":      schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		}}
}

func (r *trunkResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *trunkModel) set(t client.Trunk) {
	m.ID, m.TenantID = idString(t.ID), idString(t.TenantID)
	m.Name, m.Tech, m.Host = types.StringValue(t.Name), types.StringValue(t.Tech), types.StringValue(t.Host)
	m.Port = types.Int64Null()
	if t.Port != nil {
		m.Port = types.Int64Value(int64(*t.Port))
	}
	m.Username, m.Secret = types.StringValue(t.Username), types.StringValue(t.Secret)
	m.Register, m.OutboundCID = types.BoolValue(t.Register), types.StringValue(t.OutboundCID)
	m.MaxChannels, m.ContinueOnFail = types.Int64Value(int64(t.MaxChannels)), types.BoolValue(t.ContinueOnFail)
	m.Enabled, m.AsteriskID = types.BoolValue(t.Enabled), types.StringValue(t.AsteriskID)
}

func (m trunkModel) port() *int {
	if m.Port.IsNull() || m.Port.IsUnknown() {
		return nil
	}
	return ptr(int(m.Port.ValueInt64()))
}

func (r *trunkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m trunkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.c.CreateTrunk(ctx, tenant, client.TrunkInput{
		Name: m.Name.ValueString(), Tech: m.Tech.ValueString(), Host: m.Host.ValueString(), Port: m.port(),
		Username: m.Username.ValueString(), Secret: m.Secret.ValueString(), Register: m.Register.ValueBool(),
		OutboundCID: m.OutboundCID.ValueString(), MaxChannels: int(m.MaxChannels.ValueInt64()),
		ContinueOnFail: m.ContinueOnFail.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("create trunk", err.Error())
		return
	}
	// The API has no way to create a disabled trunk.
	if !m.Enabled.ValueBool() {
		if t, err = r.c.UpdateTrunk(ctx, tenant, t.ID, client.TrunkPatch{Enabled: ptr(false)}); err != nil {
			resp.Diagnostics.AddError("disable trunk", err.Error())
			return
		}
	}
	m.set(t)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *trunkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m trunkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.c.GetTrunk(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "trunk", err, resp)
		return
	}
	m.set(t)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *trunkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m trunkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.c.UpdateTrunk(ctx, tenant, id, client.TrunkPatch{
		Host: ptr(m.Host.ValueString()), Port: m.port(), Username: ptr(m.Username.ValueString()),
		Secret: ptr(m.Secret.ValueString()), Register: ptr(m.Register.ValueBool()),
		OutboundCID: ptr(m.OutboundCID.ValueString()), MaxChannels: ptr(int(m.MaxChannels.ValueInt64())),
		ContinueOnFail: ptr(m.ContinueOnFail.ValueBool()), Enabled: ptr(m.Enabled.ValueBool()),
	})
	if err != nil {
		resp.Diagnostics.AddError("update trunk", err.Error())
		return
	}
	m.set(t)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *trunkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m trunkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("trunk", r.c.DeleteTrunk(ctx, tenant, id), resp)
}

func (r *trunkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
