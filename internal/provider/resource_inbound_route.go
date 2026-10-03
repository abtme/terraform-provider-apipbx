package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type inboundRouteResource struct{ c *client.Client }

type inboundRouteModel struct {
	ID              types.String `tfsdk:"id"`
	TenantID        types.String `tfsdk:"tenant_id"`
	DID             types.String `tfsdk:"did"`
	Description     types.String `tfsdk:"description"`
	CIDPrefix       types.String `tfsdk:"cid_prefix"`
	DestinationType types.String `tfsdk:"destination_type"`
	DestinationID   types.String `tfsdk:"destination_id"`
}

func NewInboundRouteResource() resource.Resource { return &inboundRouteResource{} }

func (r *inboundRouteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_inbound_route"
}

func (r *inboundRouteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Sends calls to a DID to a destination. Import with \"<tenant id>/<did>\".",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id":        schema.StringAttribute{Required: true, PlanModifiers: replace},
			"did":              schema.StringAttribute{Required: true, Description: "Globally unique, 3-20 digits, optional leading +.", PlanModifiers: replace},
			"description":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"cid_prefix":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"destination_type": schema.StringAttribute{Required: true, Description: "extension, ring_group, voicemail, time_condition, ivr, queue, conference or hangup."},
			"destination_id":   schema.StringAttribute{Optional: true, Description: "Id of the extension, ring group or voicemail box."},
		}}
}

func (r *inboundRouteResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *inboundRouteModel) set(rt client.InboundRoute) {
	m.ID = types.StringValue(strings.Join([]string{idString(rt.TenantID).ValueString(), rt.DID}, "/"))
	m.TenantID, m.DID = idString(rt.TenantID), types.StringValue(rt.DID)
	m.Description, m.CIDPrefix = types.StringValue(rt.Description), types.StringValue(rt.CIDPrefix)
	m.DestinationType, m.DestinationID = destination(rt.Destination)
}

func (r *inboundRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m inboundRouteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	dest := toDestination(&resp.Diagnostics, m.DestinationType, m.DestinationID)
	if resp.Diagnostics.HasError() {
		return
	}
	rt, err := r.c.CreateInboundRoute(ctx, tenant, client.InboundRouteInput{
		DID: m.DID.ValueString(), Description: m.Description.ValueString(), CIDPrefix: m.CIDPrefix.ValueString(), Destination: *dest,
	})
	if err != nil {
		resp.Diagnostics.AddError("create inbound route", err.Error())
		return
	}
	m.set(rt)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *inboundRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m inboundRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	rt, err := r.c.GetInboundRoute(ctx, tenant, m.DID.ValueString())
	if err != nil {
		readFailed(ctx, "inbound route", err, resp)
		return
	}
	m.set(rt)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *inboundRouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m inboundRouteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	dest := toDestination(&resp.Diagnostics, m.DestinationType, m.DestinationID)
	if resp.Diagnostics.HasError() {
		return
	}
	rt, err := r.c.UpdateInboundRoute(ctx, tenant, m.DID.ValueString(), client.InboundRoutePatch{
		Description: ptr(m.Description.ValueString()), CIDPrefix: ptr(m.CIDPrefix.ValueString()), Destination: dest,
	})
	if err != nil {
		resp.Diagnostics.AddError("update inbound route", err.Error())
		return
	}
	m.set(rt)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *inboundRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m inboundRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("inbound route", r.c.DeleteInboundRoute(ctx, tenant, m.DID.ValueString()), resp)
}

func (r *inboundRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tenant, did, ok := strings.Cut(req.ID, "/")
	if !ok || tenant == "" || did == "" {
		resp.Diagnostics.AddError("invalid import id", `expected "<tenant id>/<did>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), tenant)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("did"), did)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
