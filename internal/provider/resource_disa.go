package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type disaResource struct{ c *client.Client }

type disaModel struct {
	ID       types.String `tfsdk:"id"`
	TenantID types.String `tfsdk:"tenant_id"`
	Name     types.String `tfsdk:"name"`
	PinSetID types.String `tfsdk:"pin_set_id"`
	CallerID types.String `tfsdk:"caller_id"`
}

func NewDISAResource() resource.Resource { return &disaResource{} }

func (r *disaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_disa"
}

func (r *disaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Direct inward system access: a destination where the caller keys in a PIN and then dials a number, which is called the way an extension's call would be. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"name":       schema.StringAttribute{Required: true},
			"pin_set_id": schema.StringAttribute{Required: true, Description: "The PIN set (apipbx_pin_set) the caller must key in a PIN of. Required: a DISA without PINs would let anyone dial out."},
			"caller_id":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "The caller id of calls placed through it: up to 20 digits, with an optional leading +."},
		}}
}

func (r *disaResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *disaModel) set(d client.DISA) {
	m.ID, m.TenantID, m.PinSetID = idString(d.ID), idString(d.TenantID), idString(d.PinSetID)
	m.Name, m.CallerID = types.StringValue(d.Name), types.StringValue(d.CallerID)
}

func (r *disaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m disaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, pin := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "pin_set_id", m.PinSetID)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.c.CreateDISA(ctx, tenant, client.DISAInput{Name: m.Name.ValueString(), PinSetID: pin, CallerID: m.CallerID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create DISA", err.Error())
		return
	}
	m.set(d)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *disaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m disaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.c.GetDISA(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "DISA", err, resp)
		return
	}
	m.set(d)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *disaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m disaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, pin := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID), parseID(&resp.Diagnostics, "pin_set_id", m.PinSetID)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.c.UpdateDISA(ctx, tenant, id, client.DISAPatch{Name: ptr(m.Name.ValueString()), PinSetID: &pin, CallerID: ptr(m.CallerID.ValueString())})
	if err != nil {
		resp.Diagnostics.AddError("update DISA", err.Error())
		return
	}
	m.set(d)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *disaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m disaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("DISA", r.c.DeleteDISA(ctx, tenant, id), resp)
}

func (r *disaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
