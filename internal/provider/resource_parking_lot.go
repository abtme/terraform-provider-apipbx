package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type parkingLotResource struct{ c *client.Client }

type parkingLotModel struct {
	ID         types.String `tfsdk:"id"`
	TenantID   types.String `tfsdk:"tenant_id"`
	ParkNumber types.String `tfsdk:"park_number"`
	FirstSlot  types.Int64  `tfsdk:"first_slot"`
	Slots      types.Int64  `tfsdk:"slots"`
}

func NewParkingLotResource() resource.Resource { return &parkingLotResource{} }

func (r *parkingLotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parking_lot"
}

func (r *parkingLotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A tenant's call parking lot. Import with the tenant id.",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr("One lot per tenant."),
			"park_number": schema.StringAttribute{Required: true, Description: "Dial it to park a call. Reserved: no other number of the tenant can be the same."},
			"first_slot":  schema.Int64Attribute{Required: true, Description: "The first slot number; dialling a slot picks up the call parked there."},
			"slots":       schema.Int64Attribute{Required: true, Description: "How many slots (1-100)."},
		}}
}

func (r *parkingLotResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *parkingLotModel) set(tenant int64, l client.ParkingLot) {
	m.ID, m.TenantID = idString(tenant), idString(tenant)
	m.ParkNumber, m.FirstSlot, m.Slots = types.StringValue(l.ParkNumber), types.Int64Value(int64(l.FirstSlot)), types.Int64Value(int64(l.Slots))
}

func (r *parkingLotResource) apply(ctx context.Context, m *parkingLotModel, add func(string, string), tenant int64) bool {
	l, err := r.c.SetParkingLot(ctx, tenant, client.ParkingLot{ParkNumber: m.ParkNumber.ValueString(), FirstSlot: int(m.FirstSlot.ValueInt64()), Slots: int(m.Slots.ValueInt64())})
	if err != nil {
		add("set parking lot", err.Error())
		return false
	}
	m.set(tenant, l)
	return true
}

func (r *parkingLotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m parkingLotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, resp.Diagnostics.AddError, tenant) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *parkingLotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m parkingLotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, resp.Diagnostics.AddError, tenant) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *parkingLotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m parkingLotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	l, err := r.c.GetParkingLot(ctx, tenant)
	if err != nil {
		readFailed(ctx, "parking lot", err, resp)
		return
	}
	m.set(tenant, l)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *parkingLotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m parkingLotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("parking lot", r.c.DeleteParkingLot(ctx, tenant), resp)
}

func (r *parkingLotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
