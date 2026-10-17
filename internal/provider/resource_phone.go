package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type phoneResource struct{ c *client.Client }

type phoneModel struct {
	ID                types.String    `tfsdk:"id"`
	TenantID          types.String    `tfsdk:"tenant_id"`
	MAC               types.String    `tfsdk:"mac"`
	ExtensionID       types.String    `tfsdk:"extension_id"`
	Model             types.String    `tfsdk:"model"`
	ExtraExtensionIDs types.List      `tfsdk:"extra_extension_ids"`
	Keys              []phoneKeyModel `tfsdk:"keys"`
	Description       types.String    `tfsdk:"description"`
}

type phoneKeyModel struct {
	Key   types.Int64  `tfsdk:"key"`
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
	Label types.String `tfsdk:"label"`
	Line  types.Int64  `tfsdk:"line"`
}

var phoneKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"key": types.Int64Type, "type": types.StringType, "value": types.StringType, "label": types.StringType, "line": types.Int64Type}}

func NewPhoneResource() resource.Resource { return &phoneResource{} }

func (r *phoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_phone"
}

func (r *phoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A desk phone, by MAC address, that provisions itself as an extension from the PBX's /provision/<token>/ address. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"mac": schema.StringAttribute{Required: true, Description: "The phone's MAC address, with or without : - separators; stored as 12 lower-case hex digits.",
				// another phone only when the address itself changes, not its spelling (an import reads back the stored form)
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
					resp.RequiresReplace = canonicalMAC(req.PlanValue.ValueString()) != canonicalMAC(req.StateValue.ValueString())
				}, "Replaces the phone when the MAC address changes.", "Replaces the phone when the MAC address changes.")}},
			"extension_id": schema.StringAttribute{Required: true},
			"model": schema.StringAttribute{Required: true, Description: "yealink, grandstream, sangoma-s (a Sangoma S-series desk phone, up to 6 lines), " +
				"sangoma-dect (a Sangoma DB-series DECT base, a line per handset, up to 10) or generic (a JSON file for softphone apps)."},
			"extra_extension_ids": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
				Description: "Extension ids for lines 2 and up, in line order (sangoma-s and sangoma-dect only)."},
			"keys": schema.ListNestedAttribute{Optional: true, Computed: true,
				Default:     listdefault.StaticValue(types.ListValueMust(phoneKeyType, []attr.Value{})),
				Description: "Programmable keys (sangoma-s only). Empty leaves the phone's own keys alone; otherwise every key is written and a key not listed is set to none.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"key":   schema.Int64Attribute{Required: true, Description: "1-45: the line keys on every page, in order."},
					"type":  schema.StringAttribute{Required: true, Description: "line, blf (lights with the extension in value), speed_dial, voicemail (dials *97 unless value is set), park (a park slot) or dnd."},
					"value": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Digits, * # or +; none for line and dnd."},
					"label": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "At most 31 characters."},
					"line":  schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), Description: "The phone line (account) it belongs to, from 1."},
				}}},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
		}}
}

func (r *phoneResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *phoneModel) set(p client.Phone) {
	m.ID, m.TenantID, m.ExtensionID = idString(p.ID), idString(p.TenantID), idString(p.ExtensionID)
	m.MAC, m.Model, m.Description = types.StringValue(p.MAC), types.StringValue(p.Model), types.StringValue(p.Description)
	m.ExtraExtensionIDs = idListValue(p.ExtraExtensionIDs)
	m.Keys = make([]phoneKeyModel, 0, len(p.Keys))
	for _, k := range p.Keys {
		m.Keys = append(m.Keys, phoneKeyModel{types.Int64Value(int64(k.Key)), types.StringValue(k.Type), types.StringValue(k.Value), types.StringValue(k.Label), types.Int64Value(int64(k.Line))})
	}
}

func (m phoneModel) keys() []client.PhoneKey {
	out := make([]client.PhoneKey, 0, len(m.Keys))
	for _, k := range m.Keys {
		out = append(out, client.PhoneKey{Key: int(k.Key.ValueInt64()), Type: k.Type.ValueString(), Value: k.Value.ValueString(), Label: k.Label.ValueString(), Line: int(k.Line.ValueInt64())})
	}
	return out
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
	extra := idList(&resp.Diagnostics, "extra_extension_ids", m.ExtraExtensionIDs)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.CreatePhone(ctx, tenant, client.PhoneInput{MAC: m.MAC.ValueString(), ExtensionID: ext, Model: m.Model.ValueString(),
		ExtraExtensionIDs: extra, Keys: m.keys(), Description: m.Description.ValueString()})
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
	extra := idList(&resp.Diagnostics, "extra_extension_ids", m.ExtraExtensionIDs)
	if resp.Diagnostics.HasError() {
		return
	}
	keys := m.keys()
	p, err := r.c.UpdatePhone(ctx, tenant, id, client.PhonePatch{ExtensionID: &ext, Model: ptr(m.Model.ValueString()),
		ExtraExtensionIDs: &extra, Keys: &keys, Description: ptr(m.Description.ValueString())})
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
