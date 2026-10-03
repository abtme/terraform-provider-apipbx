package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type outboundRouteResource struct{ c *client.Client }

type patternModel struct {
	Prefix  types.String `tfsdk:"prefix"`
	Match   types.String `tfsdk:"match"`
	Prepend types.String `tfsdk:"prepend"`
}

type outboundRouteModel struct {
	ID       types.String   `tfsdk:"id"`
	TenantID types.String   `tfsdk:"tenant_id"`
	Name     types.String   `tfsdk:"name"`
	Position types.Int64    `tfsdk:"position"`
	Enabled  types.Bool     `tfsdk:"enabled"`
	Patterns []patternModel `tfsdk:"patterns"`
	Trunks   types.List     `tfsdk:"trunks"`
	PinSetID types.String   `tfsdk:"pin_set_id"`
}

func NewOutboundRouteResource() resource.Resource { return &outboundRouteResource{} }

func (r *outboundRouteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_outbound_route"
}

func (r *outboundRouteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sends outbound calls matching a pattern through trunks. Import with \"<tenant id>/<route id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":      schema.StringAttribute{Required: true},
			"position":  schema.Int64Attribute{Required: true, Description: "Lower matches first."},
			"enabled":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"patterns": schema.ListNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"prefix":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Digits stripped before dialling."},
				"match":   schema.StringAttribute{Required: true, Description: "Asterisk pattern: X 0-9, Z 1-9, N 2-9, [1-3], . one or more, ! zero or more."},
				"prepend": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Digits added before dialling."},
			}}},
			"trunks":     schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Trunk ids in failover order."},
			"pin_set_id": schema.StringAttribute{Optional: true, Description: "A PIN set (apipbx_pin_set): the caller must key in one of its PINs before the call is placed."},
		}}
}

func (r *outboundRouteResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *outboundRouteModel) set(rt client.OutboundRoute) {
	m.ID, m.TenantID = idString(rt.ID), idString(rt.TenantID)
	m.Name, m.Position, m.Enabled = types.StringValue(rt.Name), types.Int64Value(int64(rt.Position)), types.BoolValue(rt.Enabled)
	m.Patterns = make([]patternModel, 0, len(rt.Patterns))
	for _, p := range rt.Patterns {
		m.Patterns = append(m.Patterns, patternModel{types.StringValue(p.Prefix), types.StringValue(p.Match), types.StringValue(p.Prepend)})
	}
	m.Trunks = idListValue(rt.Trunks)
	m.PinSetID = types.StringNull()
	if rt.PinSetID != nil {
		m.PinSetID = idString(*rt.PinSetID)
	}
}

func (m outboundRouteModel) patterns() []client.Pattern {
	out := make([]client.Pattern, 0, len(m.Patterns))
	for _, p := range m.Patterns {
		out = append(out, client.Pattern{Prefix: p.Prefix.ValueString(), Match: p.Match.ValueString(), Prepend: p.Prepend.ValueString()})
	}
	return out
}

func (r *outboundRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m outboundRouteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	trunks := idList(&resp.Diagnostics, "trunk", m.Trunks)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.OutboundRouteInput{Name: m.Name.ValueString(), Position: int(m.Position.ValueInt64()), Patterns: m.patterns(), Trunks: trunks}
	if !m.PinSetID.IsNull() && !m.PinSetID.IsUnknown() {
		pin := parseID(&resp.Diagnostics, "pin_set_id", m.PinSetID)
		if resp.Diagnostics.HasError() {
			return
		}
		in.PinSetID = &pin
	}
	rt, err := r.c.CreateOutboundRoute(ctx, tenant, in)
	if err != nil {
		resp.Diagnostics.AddError("create outbound route", err.Error())
		return
	}
	// The API has no way to create a disabled route.
	if !m.Enabled.ValueBool() {
		if rt, err = r.c.UpdateOutboundRoute(ctx, tenant, rt.ID, client.OutboundRoutePatch{Enabled: ptr(false)}); err != nil {
			resp.Diagnostics.AddError("disable outbound route", err.Error())
			return
		}
	}
	m.set(rt)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *outboundRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m outboundRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	rt, err := r.c.GetOutboundRoute(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "outbound route", err, resp)
		return
	}
	m.set(rt)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *outboundRouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m outboundRouteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	trunks := idList(&resp.Diagnostics, "trunk", m.Trunks)
	if resp.Diagnostics.HasError() {
		return
	}
	pats := m.patterns()
	pin := int64(0) // none: removes the requirement
	if !m.PinSetID.IsNull() && !m.PinSetID.IsUnknown() {
		pin = parseID(&resp.Diagnostics, "pin_set_id", m.PinSetID)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	rt, err := r.c.UpdateOutboundRoute(ctx, tenant, id, client.OutboundRoutePatch{
		Name: ptr(m.Name.ValueString()), Position: ptr(int(m.Position.ValueInt64())), Enabled: ptr(m.Enabled.ValueBool()),
		Patterns: &pats, Trunks: &trunks, PinSetID: &pin,
	})
	if err != nil {
		resp.Diagnostics.AddError("update outbound route", err.Error())
		return
	}
	m.set(rt)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *outboundRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m outboundRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("outbound route", r.c.DeleteOutboundRoute(ctx, tenant, id), resp)
}

func (r *outboundRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
