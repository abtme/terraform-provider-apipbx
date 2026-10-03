package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type ringGroupResource struct{ c *client.Client }

type ringGroupModel struct {
	ID         types.String `tfsdk:"id"`
	TenantID   types.String `tfsdk:"tenant_id"`
	Number     types.String `tfsdk:"number"`
	Name       types.String `tfsdk:"name"`
	Strategy   types.String `tfsdk:"strategy"`
	RingTime   types.Int64  `tfsdk:"ring_time"`
	Members    types.List   `tfsdk:"members"`
	FailoverTo types.String `tfsdk:"failover_type"`
	FailoverID types.String `tfsdk:"failover_id"`
}

func NewRingGroupResource() resource.Resource { return &ringGroupResource{} }

func (r *ringGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ring_group"
}

func (r *ringGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A ring group. Import with \"<tenant id>/<ring group id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"number":    schema.StringAttribute{Required: true, PlanModifiers: replace},
			"name":      schema.StringAttribute{Required: true},
			"strategy":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ringall"), Description: "ringall, hunt or firstavailable."},
			"ring_time": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(20)},
			"members":   schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Extension ids, in ring order."},
			"failover_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("hangup"),
				Description: "Where an unanswered call goes: extension, ring_group, voicemail, time_condition, ivr, queue or hangup."},
			"failover_id": schema.StringAttribute{Optional: true, Description: "Id of the failover extension, ring group or voicemail box."},
		}}
}

func (r *ringGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *ringGroupModel) set(g client.RingGroup) {
	m.ID, m.TenantID = idString(g.ID), idString(g.TenantID)
	m.Number, m.Name, m.Strategy = types.StringValue(g.Number), types.StringValue(g.Name), types.StringValue(g.Strategy)
	m.RingTime, m.Members = types.Int64Value(int64(g.RingTime)), idListValue(g.Members)
	m.FailoverTo, m.FailoverID = destination(g.Failover)
}

// destination splits an API destination into the (type, id) attribute pair.
func destination(d client.Destination) (types.String, types.String) {
	if d.ID == nil {
		return types.StringValue(d.Type), types.StringNull()
	}
	return types.StringValue(d.Type), idString(*d.ID)
}

func (r *ringGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m ringGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	members := idList(&resp.Diagnostics, "member", m.Members)
	fo := toDestination(&resp.Diagnostics, m.FailoverTo, m.FailoverID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.CreateRingGroup(ctx, tenant, client.RingGroupInput{
		Number: m.Number.ValueString(), Name: m.Name.ValueString(), Strategy: m.Strategy.ValueString(),
		RingTime: int(m.RingTime.ValueInt64()), Members: members, Failover: fo,
	})
	if err != nil {
		resp.Diagnostics.AddError("create ring group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *ringGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m ringGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.GetRingGroup(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "ring group", err, resp)
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *ringGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m ringGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	members := idList(&resp.Diagnostics, "member", m.Members)
	fo := toDestination(&resp.Diagnostics, m.FailoverTo, m.FailoverID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.UpdateRingGroup(ctx, tenant, id, client.RingGroupPatch{
		Name: ptr(m.Name.ValueString()), Strategy: ptr(m.Strategy.ValueString()), RingTime: ptr(int(m.RingTime.ValueInt64())),
		Members: &members, Failover: fo,
	})
	if err != nil {
		resp.Diagnostics.AddError("update ring group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *ringGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m ringGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("ring group", r.c.DeleteRingGroup(ctx, tenant, id), resp)
}

func (r *ringGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
