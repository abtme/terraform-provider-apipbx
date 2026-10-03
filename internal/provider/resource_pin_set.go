package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type pinSetResource struct{ c *client.Client }

type pinSetModel struct {
	ID       types.String `tfsdk:"id"`
	TenantID types.String `tfsdk:"tenant_id"`
	Name     types.String `tfsdk:"name"`
	Pins     types.Set    `tfsdk:"pins"`
}

func NewPinSetResource() resource.Resource { return &pinSetResource{} }

func (r *pinSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pin_set"
}

func (r *pinSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A set of PINs an outbound route can require. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"name": schema.StringAttribute{Required: true},
			"pins": schema.SetAttribute{Required: true, ElementType: types.StringType, Sensitive: true,
				Description: "PINs of 3-12 digits; any one of them lets a call through a route that requires this set."},
		}}
}

func (r *pinSetResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *pinSetModel) set(g client.PinSet) {
	m.ID, m.TenantID = idString(g.ID), idString(g.TenantID)
	m.Name = types.StringValue(g.Name)
	pins := make([]attr.Value, 0, len(g.Pins))
	for _, pin := range g.Pins {
		pins = append(pins, types.StringValue(pin))
	}
	m.Pins = types.SetValueMust(types.StringType, pins)
}

func (r *pinSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m pinSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), pinList(m.Pins)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.CreatePinSet(ctx, tenant, client.PinSetInput{Name: m.Name.ValueString(), Pins: members})
	if err != nil {
		resp.Diagnostics.AddError("create PIN set", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *pinSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m pinSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.GetPinSet(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "PIN set", err, resp)
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *pinSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m pinSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, members := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID), pinList(m.Pins)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.UpdatePinSet(ctx, tenant, id, client.PinSetPatch{Name: ptr(m.Name.ValueString()), Pins: &members})
	if err != nil {
		resp.Diagnostics.AddError("update PIN set", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *pinSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m pinSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("PIN set", r.c.DeletePinSet(ctx, tenant, id), resp)
}

func (r *pinSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}

func pinList(l types.Set) []string {
	var out []string
	for _, e := range l.Elements() {
		out = append(out, e.(types.String).ValueString())
	}
	return out
}
