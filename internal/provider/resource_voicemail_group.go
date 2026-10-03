package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type voicemailGroupResource struct{ c *client.Client }

type voicemailGroupModel struct {
	ID       types.String `tfsdk:"id"`
	TenantID types.String `tfsdk:"tenant_id"`
	Number   types.String `tfsdk:"number"`
	Name     types.String `tfsdk:"name"`
	Boxes    types.List   `tfsdk:"boxes"`
}

func NewVoicemailGroupResource() resource.Resource { return &voicemailGroupResource{} }

func (r *voicemailGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_voicemail_group"
}

func (r *voicemailGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A voicemail group: a message left for its number (or a route that sends callers to it) goes to every mailbox in it. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"number": replaceAttr("Dialable; shares the number space with extensions, ring groups, queues and conferences."),
			"name":   schema.StringAttribute{Required: true},
			"boxes":  schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Voicemail box ids; a message left for the group goes to each of them."},
		}}
}

func (r *voicemailGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *voicemailGroupModel) set(g client.VoicemailGroup) {
	m.ID, m.TenantID = idString(g.ID), idString(g.TenantID)
	m.Number, m.Name = types.StringValue(g.Number), types.StringValue(g.Name)
	m.Boxes = idListValue(g.Boxes)
}

func (r *voicemailGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m voicemailGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), idList(&resp.Diagnostics, "box", m.Boxes)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.CreateVoicemailGroup(ctx, tenant, client.VoicemailGroupInput{Number: m.Number.ValueString(), Name: m.Name.ValueString(), Boxes: members})
	if err != nil {
		resp.Diagnostics.AddError("create voicemail group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *voicemailGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m voicemailGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.GetVoicemailGroup(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "voicemail group", err, resp)
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *voicemailGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m voicemailGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID), idList(&resp.Diagnostics, "box", m.Boxes)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.UpdateVoicemailGroup(ctx, tenant, id, client.VoicemailGroupPatch{Name: ptr(m.Name.ValueString()), Boxes: &members})
	if err != nil {
		resp.Diagnostics.AddError("update voicemail group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *voicemailGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m voicemailGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("voicemail group", r.c.DeleteVoicemailGroup(ctx, tenant, id), resp)
}

func (r *voicemailGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
