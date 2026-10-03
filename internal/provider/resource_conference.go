package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type conferenceResource struct{ c *client.Client }

type conferenceModel struct {
	ID         types.String `tfsdk:"id"`
	TenantID   types.String `tfsdk:"tenant_id"`
	Number     types.String `tfsdk:"number"`
	Name       types.String `tfsdk:"name"`
	PIN        types.String `tfsdk:"pin"`
	AdminPIN   types.String `tfsdk:"admin_pin"`
	MaxMembers types.Int64  `tfsdk:"max_members"`
	MuteOnJoin types.Bool   `tfsdk:"mute_on_join"`
}

func NewConferenceResource() resource.Resource { return &conferenceResource{} }

func (r *conferenceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_conference"
}

func (r *conferenceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A conference room. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"number":    schema.StringAttribute{Required: true, Description: "Dialable; shares the number space with extensions, ring groups and queues.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":      schema.StringAttribute{Required: true},
			"pin": schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, Default: stringdefault.StaticString(""),
				Description: "Callers must enter it (up to 10 digits); empty leaves the room open."},
			"admin_pin": schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, Default: stringdefault.StaticString(""),
				Description: "Entering it makes a caller an admin (can lock the room and kick the last caller); must differ from pin."},
			"max_members":  schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "0 is no limit."},
			"mute_on_join": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		}}
}

func (r *conferenceResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *conferenceModel) set(c client.Conference) {
	m.ID, m.TenantID = idString(c.ID), idString(c.TenantID)
	m.Number, m.Name = types.StringValue(c.Number), types.StringValue(c.Name)
	m.PIN, m.AdminPIN = types.StringValue(c.PIN), types.StringValue(c.AdminPIN)
	m.MaxMembers, m.MuteOnJoin = types.Int64Value(int64(c.MaxMembers)), types.BoolValue(c.MuteOnJoin)
}

func (r *conferenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m conferenceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.CreateConference(ctx, tenant, client.ConferenceInput{Number: m.Number.ValueString(), Name: m.Name.ValueString(),
		PIN: m.PIN.ValueString(), AdminPIN: m.AdminPIN.ValueString(), MaxMembers: int(m.MaxMembers.ValueInt64()), MuteOnJoin: m.MuteOnJoin.ValueBool()})
	if err != nil {
		resp.Diagnostics.AddError("create conference", err.Error())
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *conferenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m conferenceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.GetConference(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "conference", err, resp)
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *conferenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m conferenceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.UpdateConference(ctx, tenant, id, client.ConferencePatch{Name: ptr(m.Name.ValueString()), PIN: ptr(m.PIN.ValueString()),
		AdminPIN: ptr(m.AdminPIN.ValueString()), MaxMembers: ptr(int(m.MaxMembers.ValueInt64())), MuteOnJoin: ptr(m.MuteOnJoin.ValueBool())})
	if err != nil {
		resp.Diagnostics.AddError("update conference", err.Error())
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *conferenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m conferenceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("conference", r.c.DeleteConference(ctx, tenant, id), resp)
}

func (r *conferenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
