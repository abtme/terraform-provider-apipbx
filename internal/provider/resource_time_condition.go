package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type timeConditionResource struct{ c *client.Client }

type timeConditionModel struct {
	ID          types.String `tfsdk:"id"`
	TenantID    types.String `tfsdk:"tenant_id"`
	Name        types.String `tfsdk:"name"`
	TimeGroupID types.String `tfsdk:"time_group_id"`
	MatchType   types.String `tfsdk:"match_type"`
	MatchID     types.String `tfsdk:"match_id"`
	NoMatchType types.String `tfsdk:"no_match_type"`
	NoMatchID   types.String `tfsdk:"no_match_id"`
	Override    types.String `tfsdk:"override"`
	MatchesNow  types.Bool   `tfsdk:"matches_now"`
}

func NewTimeConditionResource() resource.Resource { return &timeConditionResource{} }

func (r *timeConditionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_time_condition"
}

func (r *timeConditionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	dest := "extension, ring_group, voicemail, time_condition, ivr or hangup."
	resp.Schema = schema.Schema{
		Description: "Sends calls to one destination while a time group matches and to another otherwise. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id":     schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":          schema.StringAttribute{Required: true},
			"time_group_id": schema.StringAttribute{Required: true},
			"match_type":    schema.StringAttribute{Required: true, Description: "Destination while the group matches: " + dest},
			"match_id":      schema.StringAttribute{Optional: true, Description: "Id of that destination (not for hangup)."},
			"no_match_type": schema.StringAttribute{Required: true, Description: "Destination otherwise: " + dest},
			"no_match_id":   schema.StringAttribute{Optional: true},
			"override": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("none"),
				Description: "none, match or no_match: force one branch whatever the time."},
			"matches_now": schema.BoolAttribute{Computed: true, Description: "Whether the time group matches at the moment of the last refresh (informational)."},
		}}
}

func (r *timeConditionResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *timeConditionModel) set(c client.TimeCondition) {
	m.ID, m.TenantID, m.TimeGroupID = idString(c.ID), idString(c.TenantID), idString(c.TimeGroupID)
	m.Name, m.Override, m.MatchesNow = types.StringValue(c.Name), types.StringValue(c.Override), types.BoolValue(c.MatchesNow)
	m.MatchType, m.MatchID = destination(c.Match)
	m.NoMatchType, m.NoMatchID = destination(c.NoMatch)
}

func (r *timeConditionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m timeConditionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, group := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "time_group_id", m.TimeGroupID)
	match := toDestination(&resp.Diagnostics, m.MatchType, m.MatchID)
	noMatch := toDestination(&resp.Diagnostics, m.NoMatchType, m.NoMatchID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.CreateTimeCondition(ctx, tenant, client.TimeConditionInput{Name: m.Name.ValueString(), TimeGroupID: group,
		Match: *match, NoMatch: *noMatch, Override: m.Override.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create time condition", err.Error())
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *timeConditionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m timeConditionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.GetTimeCondition(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "time condition", err, resp)
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *timeConditionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m timeConditionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, group := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID),
		parseID(&resp.Diagnostics, "time_group_id", m.TimeGroupID)
	match := toDestination(&resp.Diagnostics, m.MatchType, m.MatchID)
	noMatch := toDestination(&resp.Diagnostics, m.NoMatchType, m.NoMatchID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.UpdateTimeCondition(ctx, tenant, id, client.TimeConditionPatch{Name: ptr(m.Name.ValueString()), TimeGroupID: &group,
		Match: match, NoMatch: noMatch, Override: ptr(m.Override.ValueString())})
	if err != nil {
		resp.Diagnostics.AddError("update time condition", err.Error())
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *timeConditionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m timeConditionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("time condition", r.c.DeleteTimeCondition(ctx, tenant, id), resp)
}

func (r *timeConditionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
