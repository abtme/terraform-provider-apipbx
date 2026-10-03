package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type extensionResource struct{ c *client.Client }

type extensionModel struct {
	ID          types.String `tfsdk:"id"`
	TenantID    types.String `tfsdk:"tenant_id"`
	Number      types.String `tfsdk:"number"`
	Name        types.String `tfsdk:"name"`
	Tech        types.String `tfsdk:"tech"`
	Secret      types.String `tfsdk:"secret"`
	Username    types.String `tfsdk:"username"`
	OutboundCID types.String `tfsdk:"outbound_cid"`
	RingTime    types.Int64  `tfsdk:"ring_time"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

func NewExtensionResource() resource.Resource { return &extensionResource{} }

func (r *extensionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_extension"
}

func (r *extensionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A phone extension (SIP or IAX2). Import with \"<tenant id>/<extension id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"number":    schema.StringAttribute{Required: true, Description: "2-16 characters from 0-9 * #.", PlanModifiers: replace},
			"name":      schema.StringAttribute{Required: true},
			"tech": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("pjsip"),
				Description: "pjsip or iax2.", PlanModifiers: replace},
			"secret": schema.StringAttribute{Optional: true, Computed: true, Sensitive: true,
				Description:   "The phone's password; generated when omitted.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"username":     schema.StringAttribute{Computed: true, Description: "What the phone authenticates as.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"outbound_cid": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"ring_time":    schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(20)},
			"enabled":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		}}
}

func (r *extensionResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *extensionModel) set(e client.Extension) {
	m.ID, m.TenantID = idString(e.ID), idString(e.TenantID)
	m.Number, m.Name, m.Tech = types.StringValue(e.Number), types.StringValue(e.Name), types.StringValue(e.Tech)
	m.Secret, m.Username = types.StringValue(e.Secret), types.StringValue(e.Username)
	m.OutboundCID = types.StringValue(e.OutboundCID)
	m.RingTime, m.Enabled = types.Int64Value(int64(e.RingTime)), types.BoolValue(e.Enabled)
}

func (r *extensionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m extensionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.c.CreateExtension(ctx, tenant, client.ExtensionInput{
		Number: m.Number.ValueString(), Name: m.Name.ValueString(), Tech: m.Tech.ValueString(),
		Secret: m.Secret.ValueString(), OutboundCID: m.OutboundCID.ValueString(), RingTime: int(m.RingTime.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError("create extension", err.Error())
		return
	}
	// The API has no way to create a disabled extension.
	if !m.Enabled.ValueBool() {
		if e, err = r.c.UpdateExtension(ctx, tenant, e.ID, client.ExtensionPatch{Enabled: ptr(false)}); err != nil {
			resp.Diagnostics.AddError("disable extension", err.Error())
			return
		}
	}
	m.set(e)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *extensionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m extensionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.c.GetExtension(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "extension", err, resp)
		return
	}
	m.set(e)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *extensionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m extensionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.c.UpdateExtension(ctx, tenant, id, client.ExtensionPatch{
		Name: ptr(m.Name.ValueString()), Secret: ptr(m.Secret.ValueString()), OutboundCID: ptr(m.OutboundCID.ValueString()),
		RingTime: ptr(int(m.RingTime.ValueInt64())), Enabled: ptr(m.Enabled.ValueBool()),
	})
	if err != nil {
		resp.Diagnostics.AddError("update extension", err.Error())
		return
	}
	m.set(e)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *extensionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m extensionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("extension", r.c.DeleteExtension(ctx, tenant, id), resp)
}

func (r *extensionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
