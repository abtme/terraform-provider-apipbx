package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type mohClassResource struct{ c *client.Client }

type mohClassModel struct {
	ID         types.String `tfsdk:"id"`
	TenantID   types.String `tfsdk:"tenant_id"`
	Name       types.String `tfsdk:"name"`
	Random     types.Bool   `tfsdk:"random"`
	Recordings types.List   `tfsdk:"recordings"`
}

func NewMohClassResource() resource.Resource { return &mohClassResource{} }

func (r *mohClassResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_moh_class"
}

func (r *mohClassResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A music on hold class: recordings played in order (or shuffled). A queue plays it by naming it in music_on_hold. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"name":       replaceAttr("Letters, digits, _ and -; a queue's music_on_hold names it."),
			"random":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Play the recordings in random order."},
			"recordings": schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Recording ids, played in this order."},
		}}
}

func (r *mohClassResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *mohClassModel) set(g client.MohClass) {
	m.ID, m.TenantID = idString(g.ID), idString(g.TenantID)
	m.Name, m.Random = types.StringValue(g.Name), types.BoolValue(g.Random)
	m.Recordings = idListValue(g.Recordings)
}

func (r *mohClassResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m mohClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), idList(&resp.Diagnostics, "recording", m.Recordings)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.CreateMohClass(ctx, tenant, client.MohClassInput{Name: m.Name.ValueString(), Random: m.Random.ValueBool(), Recordings: members})
	if err != nil {
		resp.Diagnostics.AddError("create music on hold class", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *mohClassResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m mohClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.GetMohClass(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "music on hold class", err, resp)
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *mohClassResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m mohClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID), idList(&resp.Diagnostics, "recording", m.Recordings)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.UpdateMohClass(ctx, tenant, id, client.MohClassPatch{Random: ptr(m.Random.ValueBool()), Recordings: &members})
	if err != nil {
		resp.Diagnostics.AddError("update music on hold class", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *mohClassResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m mohClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("music on hold class", r.c.DeleteMohClass(ctx, tenant, id), resp)
}

func (r *mohClassResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
