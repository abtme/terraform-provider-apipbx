package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type callLimitResource struct{ c *client.Client }

type callLimitModel struct {
	ID       types.String `tfsdk:"id"`
	TenantID types.String `tfsdk:"tenant_id"`
	MaxCalls types.Int64  `tfsdk:"max_calls"`
}

func NewCallLimitResource() resource.Resource { return &callLimitResource{} }

func (r *callLimitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_call_limit"
}

func (r *callLimitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A tenant's limit on calls in progress at once. Import with the tenant id.",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr("One lot per tenant."),
			"max_calls": schema.Int64Attribute{Required: true, Description: "The most calls the tenant may have in progress at once (1-100000); one over it gets a busy signal."},
		}}
}

func (r *callLimitResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *callLimitModel) set(tenant int64, l client.CallLimit) {
	m.ID, m.TenantID = idString(tenant), idString(tenant)
	m.MaxCalls = types.Int64Value(int64(l.MaxCalls))
}

func (r *callLimitResource) apply(ctx context.Context, m *callLimitModel, add func(string, string), tenant int64) bool {
	l, err := r.c.SetCallLimit(ctx, tenant, client.CallLimit{MaxCalls: int(m.MaxCalls.ValueInt64())})
	if err != nil {
		add("set call limit", err.Error())
		return false
	}
	m.set(tenant, l)
	return true
}

func (r *callLimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m callLimitModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, resp.Diagnostics.AddError, tenant) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *callLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m callLimitModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, resp.Diagnostics.AddError, tenant) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *callLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m callLimitModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	l, err := r.c.GetCallLimit(ctx, tenant)
	if err != nil {
		readFailed(ctx, "call limit", err, resp)
		return
	}
	m.set(tenant, l)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *callLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m callLimitModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("call limit", r.c.DeleteCallLimit(ctx, tenant), resp)
}

func (r *callLimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
