package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type phoneResource struct{ c *client.Client }

type phoneModel struct {
	ID          types.String `tfsdk:"id"`
	TenantID    types.String `tfsdk:"tenant_id"`
	MAC         types.String `tfsdk:"mac"`
	ExtensionID types.String `tfsdk:"extension_id"`
	Model       types.String `tfsdk:"model"`
	Description types.String `tfsdk:"description"`
}

func NewPhoneResource() resource.Resource { return &phoneResource{} }

func (r *phoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_phone"
}

func (r *phoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A desk phone, by MAC address, that provisions itself as an extension from the PBX's /provision/<token>/ address. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"mac":          replaceAttr("The phone's MAC address, with or without : - separators; stored as 12 lower-case hex digits."),
			"extension_id": schema.StringAttribute{Required: true},
			"model":        schema.StringAttribute{Required: true, Description: "yealink, grandstream or generic (a JSON file for softphone apps)."},
			"description":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
		}}
}

func (r *phoneResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *phoneModel) set(p client.Phone) {
	m.ID, m.TenantID, m.ExtensionID = idString(p.ID), idString(p.TenantID), idString(p.ExtensionID)
	m.MAC, m.Model, m.Description = types.StringValue(p.MAC), types.StringValue(p.Model), types.StringValue(p.Description)
}

// canonicalMAC is how the API stores a MAC: the configuration may spell it any way.
func canonicalMAC(s string) string {
	s = strings.ToLower(s)
	return strings.NewReplacer(":", "", "-", "", ".", "").Replace(s)
}

func (r *phoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m phoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.CreatePhone(ctx, tenant, client.PhoneInput{MAC: m.MAC.ValueString(), ExtensionID: ext, Model: m.Model.ValueString(), Description: m.Description.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create phone", err.Error())
		return
	}
	configured := m.MAC
	m.set(p)
	if canonicalMAC(configured.ValueString()) == p.MAC {
		m.MAC = configured // keep the spelling the configuration uses
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *phoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m phoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.GetPhone(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "phone", err, resp)
		return
	}
	configured := m.MAC
	m.set(p)
	if !configured.IsNull() && canonicalMAC(configured.ValueString()) == p.MAC {
		m.MAC = configured
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *phoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m phoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.UpdatePhone(ctx, tenant, id, client.PhonePatch{ExtensionID: &ext, Model: ptr(m.Model.ValueString()), Description: ptr(m.Description.ValueString())})
	if err != nil {
		resp.Diagnostics.AddError("update phone", err.Error())
		return
	}
	configured := m.MAC
	m.set(p)
	m.MAC = configured
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *phoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m phoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("phone", r.c.DeletePhone(ctx, tenant, id), resp)
}

func (r *phoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
