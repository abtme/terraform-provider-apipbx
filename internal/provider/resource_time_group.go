package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type timeGroupResource struct{ c *client.Client }

type timeRangeModel struct {
	Start     types.String `tfsdk:"start"`
	End       types.String `tfsdk:"end"`
	Weekdays  types.List   `tfsdk:"weekdays"`
	MonthDays types.List   `tfsdk:"month_days"`
	Months    types.List   `tfsdk:"months"`
}

type timeGroupModel struct {
	ID       types.String     `tfsdk:"id"`
	TenantID types.String     `tfsdk:"tenant_id"`
	Name     types.String     `tfsdk:"name"`
	Timezone types.String     `tfsdk:"timezone"`
	Ranges   []timeRangeModel `tfsdk:"ranges"`
}

func NewTimeGroupResource() resource.Resource { return &timeGroupResource{} }

func (r *timeGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_time_group"
}

func emptyList(t attr.Type) defaults.List {
	return listdefault.StaticValue(types.ListValueMust(t, []attr.Value{}))
}

func (r *timeGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A set of time ranges. Import with \"<tenant id>/<time group id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":      schema.StringAttribute{Required: true},
			"timezone":  schema.StringAttribute{Required: true, Description: "IANA name such as Europe/London; daylight saving is followed."},
			"ranges": schema.ListNestedAttribute{Required: true, Description: "The group matches when any range does.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"start": schema.StringAttribute{Required: true, Description: "HH:MM, inclusive."},
					"end":   schema.StringAttribute{Required: true, Description: "HH:MM, exclusive; 24:00 is the end of the day. An end before the start runs over midnight and belongs to the days it starts on."},
					"weekdays": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: emptyList(types.StringType),
						Description: "sun, mon, tue, wed, thu, fri, sat; empty means every day."},
					"month_days": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.Int64Type, Default: emptyList(types.Int64Type),
						Description: "1-31; empty means every day of the month."},
					"months": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.Int64Type, Default: emptyList(types.Int64Type),
						Description: "1-12; empty means every month."},
				}}},
		}}
}

func (r *timeGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func intsValue(v []int) types.List {
	elems := make([]attr.Value, 0, len(v))
	for _, n := range v {
		elems = append(elems, types.Int64Value(int64(n)))
	}
	return types.ListValueMust(types.Int64Type, elems)
}

func stringsValue(v []string) types.List {
	elems := make([]attr.Value, 0, len(v))
	for _, s := range v {
		elems = append(elems, types.StringValue(s))
	}
	return types.ListValueMust(types.StringType, elems)
}

func (m *timeGroupModel) set(g client.TimeGroup) {
	m.ID, m.TenantID = idString(g.ID), idString(g.TenantID)
	m.Name, m.Timezone = types.StringValue(g.Name), types.StringValue(g.Timezone)
	m.Ranges = make([]timeRangeModel, 0, len(g.Ranges))
	for _, r := range g.Ranges {
		m.Ranges = append(m.Ranges, timeRangeModel{types.StringValue(r.Start), types.StringValue(r.End),
			stringsValue(r.Weekdays), intsValue(r.MonthDays), intsValue(r.Months)})
	}
}

func (m timeGroupModel) ranges() []client.TimeRange {
	out := make([]client.TimeRange, 0, len(m.Ranges))
	for _, r := range m.Ranges {
		tr := client.TimeRange{Start: r.Start.ValueString(), End: r.End.ValueString(), Weekdays: []string{}, MonthDays: []int{}, Months: []int{}}
		for _, e := range r.Weekdays.Elements() {
			tr.Weekdays = append(tr.Weekdays, e.(types.String).ValueString())
		}
		for _, e := range r.MonthDays.Elements() {
			tr.MonthDays = append(tr.MonthDays, int(e.(types.Int64).ValueInt64()))
		}
		for _, e := range r.Months.Elements() {
			tr.Months = append(tr.Months, int(e.(types.Int64).ValueInt64()))
		}
		out = append(out, tr)
	}
	return out
}

func (r *timeGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m timeGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.CreateTimeGroup(ctx, tenant, client.TimeGroupInput{Name: m.Name.ValueString(), Timezone: m.Timezone.ValueString(), Ranges: m.ranges()})
	if err != nil {
		resp.Diagnostics.AddError("create time group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *timeGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m timeGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.c.GetTimeGroup(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "time group", err, resp)
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *timeGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m timeGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	rs := m.ranges()
	g, err := r.c.UpdateTimeGroup(ctx, tenant, id, client.TimeGroupPatch{Name: ptr(m.Name.ValueString()), Timezone: ptr(m.Timezone.ValueString()), Ranges: &rs})
	if err != nil {
		resp.Diagnostics.AddError("update time group", err.Error())
		return
	}
	m.set(g)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *timeGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m timeGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("time group", r.c.DeleteTimeGroup(ctx, tenant, id), resp)
}

func (r *timeGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
