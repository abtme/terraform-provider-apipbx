package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type webhookResource struct{ c *client.Client }

type webhookModel struct {
	ID       types.String `tfsdk:"id"`
	TenantID types.String `tfsdk:"tenant_id"`
	URL      types.String `tfsdk:"url"`
	Secret   types.String `tfsdk:"secret"`
	Events   types.Set    `tfsdk:"events"`
	Enabled  types.Bool   `tfsdk:"enabled"`
}

func NewWebhookResource() resource.Resource { return &webhookResource{} }

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Posts a tenant's call.ended and voicemail.received events to a URL, signed with the secret (header X-Apipbx-Signature: sha256=<hex HMAC-SHA256 of the body>). Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"url": schema.StringAttribute{Required: true, Description: "An http or https address. Private addresses are refused unless the platform allows them."},
			"secret": schema.StringAttribute{Optional: true, Computed: true, Sensitive: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "16-128 characters; generated when left out."},
			"events": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				Description:   "call.ended and/or voicemail.received; both when left out."},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		}}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *webhookModel) set(w client.Webhook) {
	m.ID, m.TenantID = idString(w.ID), idString(w.TenantID)
	m.URL, m.Secret, m.Enabled = types.StringValue(w.URL), types.StringValue(w.Secret), types.BoolValue(w.Enabled)
	ev := make([]attr.Value, 0, len(w.Events))
	for _, e := range w.Events {
		ev = append(ev, types.StringValue(e))
	}
	m.Events = types.SetValueMust(types.StringType, ev)
}

func (m webhookModel) events() []string {
	var out []string
	for _, e := range m.Events.Elements() {
		out = append(out, e.(types.String).ValueString())
	}
	return out
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.WebhookInput{URL: m.URL.ValueString(), Enabled: ptr(m.Enabled.ValueBool())}
	if !m.Secret.IsUnknown() && !m.Secret.IsNull() {
		in.Secret = m.Secret.ValueString()
	}
	if !m.Events.IsUnknown() && !m.Events.IsNull() {
		in.Events = m.events()
	}
	w, err := r.c.CreateWebhook(ctx, tenant, in)
	if err != nil {
		resp.Diagnostics.AddError("create webhook", err.Error())
		return
	}
	m.set(w)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	w, err := r.c.GetWebhook(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "webhook", err, resp)
		return
	}
	m.set(w)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	ev := m.events()
	w, err := r.c.UpdateWebhook(ctx, tenant, id, client.WebhookPatch{URL: ptr(m.URL.ValueString()), Secret: ptr(m.Secret.ValueString()), Events: &ev, Enabled: ptr(m.Enabled.ValueBool())})
	if err != nil {
		resp.Diagnostics.AddError("update webhook", err.Error())
		return
	}
	m.set(w)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("webhook", r.c.DeleteWebhook(ctx, tenant, id), resp)
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
