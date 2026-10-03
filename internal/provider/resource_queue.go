package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type queueResource struct{ c *client.Client }

type queueMemberModel struct {
	ExtensionID types.String `tfsdk:"extension_id"`
	Penalty     types.Int64  `tfsdk:"penalty"`
	Dynamic     types.Bool   `tfsdk:"dynamic"`
}

type queueModel struct {
	ID                types.String       `tfsdk:"id"`
	TenantID          types.String       `tfsdk:"tenant_id"`
	Number            types.String       `tfsdk:"number"`
	Name              types.String       `tfsdk:"name"`
	Strategy          types.String       `tfsdk:"strategy"`
	AgentTimeout      types.Int64        `tfsdk:"agent_timeout"`
	Retry             types.Int64        `tfsdk:"retry"`
	WrapupTime        types.Int64        `tfsdk:"wrapup_time"`
	MaxWait           types.Int64        `tfsdk:"max_wait"`
	MaxCallers        types.Int64        `tfsdk:"max_callers"`
	JoinEmpty         types.Bool         `tfsdk:"join_empty"`
	LeaveWhenEmpty    types.Bool         `tfsdk:"leave_when_empty"`
	AnnouncePosition  types.Bool         `tfsdk:"announce_position"`
	AnnounceHoldTime  types.Bool         `tfsdk:"announce_hold_time"`
	AnnounceFrequency types.Int64        `tfsdk:"announce_frequency"`
	MusicOnHold       types.String       `tfsdk:"music_on_hold"`
	Members           []queueMemberModel `tfsdk:"members"`
	TimeoutType       types.String       `tfsdk:"timeout_type"`
	TimeoutID         types.String       `tfsdk:"timeout_id"`
}

func NewQueueResource() resource.Resource { return &queueResource{} }

func (r *queueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_queue"
}

func (r *queueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A call queue. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"number":    schema.StringAttribute{Required: true, Description: "Dialable; shares the number space with extensions and ring groups.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":      schema.StringAttribute{Required: true},
			"strategy": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ringall"),
				Description: "ringall, leastrecent, fewestcalls, random, rrmemory, rrordered, linear or wrandom."},
			"agent_timeout":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(15), Description: "Seconds to ring one agent (1-300)."},
			"retry":              schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(5), Description: "Seconds before the agents are tried again (0-300)."},
			"wrapup_time":        schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "Seconds an agent rests after a call."},
			"max_wait":           schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "Seconds a caller may wait; 0 is for ever."},
			"max_callers":        schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "Callers allowed to wait at once; 0 is no limit."},
			"join_empty":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Callers may join when no agent can take the call."},
			"leave_when_empty":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Waiting callers leave when the last able agent goes."},
			"announce_position":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"announce_hold_time": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"announce_frequency": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "Seconds between announcements; needed when a position or hold time is announced."},
			"music_on_hold":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("default"), Description: "A music on hold class on the Asterisk host."},
			"members": schema.ListNestedAttribute{Optional: true, Computed: true, Description: "Agents. Use PJSIP extensions: IAX2 phones report no device state, so queues do not ring them reliably.",
				Default: listdefault.StaticValue(types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
					"extension_id": types.StringType, "penalty": types.Int64Type, "dynamic": types.BoolType}}, []attr.Value{})),
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"extension_id": schema.StringAttribute{Required: true},
					"penalty":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "0-1000; lower is tried first."},
					"dynamic":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Only in the queue while its owner is logged in (*45)."},
				}}},
			"timeout_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("hangup"),
				Description: "Where callers go who wait too long or cannot join: extension, ring_group, voicemail, time_condition, ivr, queue or hangup."},
			"timeout_id": schema.StringAttribute{Optional: true},
		}}
}

func (r *queueResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *queueModel) set(q client.CallQueue) {
	m.ID, m.TenantID = idString(q.ID), idString(q.TenantID)
	m.Number, m.Name, m.Strategy = types.StringValue(q.Number), types.StringValue(q.Name), types.StringValue(q.Strategy)
	m.AgentTimeout, m.Retry = types.Int64Value(int64(q.AgentTimeout)), types.Int64Value(int64(q.Retry))
	m.WrapupTime, m.MaxWait, m.MaxCallers = types.Int64Value(int64(q.WrapupTime)), types.Int64Value(int64(q.MaxWait)), types.Int64Value(int64(q.MaxCallers))
	m.JoinEmpty, m.LeaveWhenEmpty = types.BoolValue(q.JoinEmpty), types.BoolValue(q.LeaveWhenEmpty)
	m.AnnouncePosition, m.AnnounceHoldTime = types.BoolValue(q.AnnouncePosition), types.BoolValue(q.AnnounceHoldTime)
	m.AnnounceFrequency, m.MusicOnHold = types.Int64Value(int64(q.AnnounceFrequency)), types.StringValue(q.MusicOnHold)
	m.Members = make([]queueMemberModel, 0, len(q.Members))
	for _, mem := range q.Members {
		m.Members = append(m.Members, queueMemberModel{idString(mem.ExtensionID), types.Int64Value(int64(mem.Penalty)), types.BoolValue(mem.Dynamic)})
	}
	m.TimeoutType, m.TimeoutID = destination(q.TimeoutDestination)
}

func (m queueModel) members(r *queueResource, d interface{ AddError(string, string) }, parse func(types.String) int64) []client.QueueMember {
	out := make([]client.QueueMember, 0, len(m.Members))
	for _, mem := range m.Members {
		out = append(out, client.QueueMember{ExtensionID: parse(mem.ExtensionID), Penalty: int(mem.Penalty.ValueInt64()), Dynamic: mem.Dynamic.ValueBool()})
	}
	return out
}

func (r *queueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m queueModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	members := m.members(r, &resp.Diagnostics, func(s types.String) int64 { return parseID(&resp.Diagnostics, "member extension_id", s) })
	dest := toDestination(&resp.Diagnostics, m.TimeoutType, m.TimeoutID)
	if resp.Diagnostics.HasError() {
		return
	}
	q, err := r.c.CreateCallQueue(ctx, tenant, client.CallQueueInput{Number: m.Number.ValueString(), Name: m.Name.ValueString(),
		Strategy: m.Strategy.ValueString(), AgentTimeout: int(m.AgentTimeout.ValueInt64()), Retry: ptr(int(m.Retry.ValueInt64())),
		WrapupTime: int(m.WrapupTime.ValueInt64()), MaxWait: int(m.MaxWait.ValueInt64()), MaxCallers: int(m.MaxCallers.ValueInt64()),
		JoinEmpty: m.JoinEmpty.ValueBool(), LeaveWhenEmpty: m.LeaveWhenEmpty.ValueBool(), AnnouncePosition: m.AnnouncePosition.ValueBool(),
		AnnounceHoldTime: m.AnnounceHoldTime.ValueBool(), AnnounceFrequency: int(m.AnnounceFrequency.ValueInt64()),
		MusicOnHold: m.MusicOnHold.ValueString(), Members: members, TimeoutDestination: dest})
	if err != nil {
		resp.Diagnostics.AddError("create queue", err.Error())
		return
	}
	m.set(q)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *queueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m queueModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	q, err := r.c.GetCallQueue(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "queue", err, resp)
		return
	}
	m.set(q)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *queueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m queueModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	members := m.members(r, &resp.Diagnostics, func(s types.String) int64 { return parseID(&resp.Diagnostics, "member extension_id", s) })
	dest := toDestination(&resp.Diagnostics, m.TimeoutType, m.TimeoutID)
	if resp.Diagnostics.HasError() {
		return
	}
	q, err := r.c.UpdateCallQueue(ctx, tenant, id, client.CallQueuePatch{Name: ptr(m.Name.ValueString()), Strategy: ptr(m.Strategy.ValueString()),
		AgentTimeout: ptr(int(m.AgentTimeout.ValueInt64())), Retry: ptr(int(m.Retry.ValueInt64())), WrapupTime: ptr(int(m.WrapupTime.ValueInt64())),
		MaxWait: ptr(int(m.MaxWait.ValueInt64())), MaxCallers: ptr(int(m.MaxCallers.ValueInt64())), JoinEmpty: ptr(m.JoinEmpty.ValueBool()),
		LeaveWhenEmpty: ptr(m.LeaveWhenEmpty.ValueBool()), AnnouncePosition: ptr(m.AnnouncePosition.ValueBool()),
		AnnounceHoldTime: ptr(m.AnnounceHoldTime.ValueBool()), AnnounceFrequency: ptr(int(m.AnnounceFrequency.ValueInt64())),
		MusicOnHold: ptr(m.MusicOnHold.ValueString()), Members: &members, TimeoutDestination: dest})
	if err != nil {
		resp.Diagnostics.AddError("update queue", err.Error())
		return
	}
	m.set(q)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *queueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m queueModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("queue", r.c.DeleteCallQueue(ctx, tenant, id), resp)
}

func (r *queueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
