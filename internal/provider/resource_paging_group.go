package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type pagingGroupResource struct{ c *client.Client }

type pagingGroupModel struct {
	ID       types.String `tfsdk:"id"`
	TenantID types.String `tfsdk:"tenant_id"`
	Number   types.String `tfsdk:"number"`
	Name     types.String `tfsdk:"name"`
	Duplex   types.Bool   `tfsdk:"duplex"`
	Members  types.List   `tfsdk:"members"`
}

func NewPagingGroupResource() resource.Resource { return &pagingGroupResource{} }

func (r *pagingGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_paging_group"
}

func (r *pagingGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A paging group: dialling its number pages the idle members, whose phones answer by themselves. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"number":  replaceAttr("Dialable; shares the number space with extensions, ring groups, queues and conferences."),
			"name":    schema.StringAttribute{Required: true},
			"duplex":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Members can talk back."},
			"members": schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Extension ids."},
		}}
}

func (r *pagingGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *pagingGroupModel) set(g client.PagingGroup) {
	m.ID, m.TenantID = idString(g.ID), idString(g.TenantID)
	m.Number, m.Name, m.Duplex = types.StringValue(g.Number), types.StringValue(g.Name), types.BoolValue(g.Duplex)
	m.Members = idListValue(g.Members)
}

func (r *pagingGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m pagingGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), idList(&resp.Diagnostics, "member", m.Members)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.CreatePagingGroup(ctx, tenant, client.PagingGroupInput{Number: m.Number.ValueString(), Name: m.Name.ValueString(), Duplex: m.Duplex.ValueBool(), Members: members})
	if err != nil {
		resp.Diagnostics.AddError("create paging group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *pagingGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m pagingGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.GetPagingGroup(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "paging group", err, resp)
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *pagingGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m pagingGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID), idList(&resp.Diagnostics, "member", m.Members)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.UpdatePagingGroup(ctx, tenant, id, client.PagingGroupPatch{Name: ptr(m.Name.ValueString()), Duplex: ptr(m.Duplex.ValueBool()), Members: &members})
	if err != nil {
		resp.Diagnostics.AddError("update paging group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *pagingGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m pagingGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("paging group", r.c.DeletePagingGroup(ctx, tenant, id), resp)
}

func (r *pagingGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
