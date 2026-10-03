package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type apiKeyResource struct{ c *client.Client }

type apiKeyModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	TenantID types.String `tfsdk:"tenant_id"`
	Key      types.String `tfsdk:"key"`
}

func NewAPIKeyResource() resource.Resource { return &apiKeyResource{} }

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An API key. The secret is only available in the state of the apply that created it; " +
			"an imported key has no secret.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"tenant_id": schema.StringAttribute{Optional: true, Description: "Scope the key to one tenant; omit for a platform key.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"key": schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		}}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m apiKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	var tenant *int64
	if !m.TenantID.IsNull() {
		tenant = ptr(parseID(&resp.Diagnostics, "tenant_id", m.TenantID))
	}
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := r.c.CreateAPIKey(ctx, m.Name.ValueString(), tenant)
	if err != nil {
		resp.Diagnostics.AddError("create api key", err.Error())
		return
	}
	m.ID, m.Key = idString(k.ID), types.StringValue(k.Key)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "api key id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := r.c.GetAPIKey(ctx, id)
	if err != nil {
		readFailed(ctx, "api key", err, resp)
		return
	}
	m.Name = types.StringValue(k.Name)
	m.TenantID = types.StringNull()
	if k.TenantID != nil {
		m.TenantID = idString(*k.TenantID)
	}
	if m.Key.IsNull() || m.Key.IsUnknown() {
		m.Key = types.StringNull() // imported: the secret cannot be recovered
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// Update is never reached: every attribute forces a new key.
func (r *apiKeyResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "api key id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("api key", r.c.DeleteAPIKey(ctx, id), resp)
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
