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

type tenantResource struct{ c *client.Client }

type tenantModel struct {
	ID   types.String `tfsdk:"id"`
	Slug types.String `tfsdk:"slug"`
	Name types.String `tfsdk:"name"`
}

func NewTenantResource() resource.Resource { return &tenantResource{} }

func (r *tenantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (r *tenantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A PBX tenant. Deleting it deletes everything in it. Needs a platform API key.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"slug": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		}}
}

func (r *tenantResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (r *tenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m tenantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.c.CreateTenant(ctx, m.Slug.ValueString(), m.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("create tenant", err.Error())
		return
	}
	m.ID = idString(t.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *tenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m tenantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "tenant id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	t, err := r.c.GetTenant(ctx, id)
	if err != nil {
		readFailed(ctx, "tenant", err, resp)
		return
	}
	m.Slug, m.Name = types.StringValue(t.Slug), types.StringValue(t.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// Update is never reached: every attribute forces a new tenant.
func (r *tenantResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *tenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m tenantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "tenant id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("tenant", r.c.DeleteTenant(ctx, id), resp)
}

func (r *tenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
