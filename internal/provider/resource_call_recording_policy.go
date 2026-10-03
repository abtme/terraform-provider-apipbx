package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type callRecordingPolicyResource struct{ c *client.Client }

type callRecordingPolicyModel struct {
	ID          types.String `tfsdk:"id"`
	TenantID    types.String `tfsdk:"tenant_id"`
	ExtensionID types.String `tfsdk:"extension_id"`
	Mode        types.String `tfsdk:"mode"`
}

func NewCallRecordingPolicyResource() resource.Resource { return &callRecordingPolicyResource{} }

func (r *callRecordingPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_call_recording_policy"
}

func (r *callRecordingPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Which of an extension's calls are recorded. Import with \"<tenant id>/<extension id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"extension_id": replaceAttr("The extension whose calls are recorded."),
			"mode": schema.StringAttribute{Required: true,
				Description: "inbound (calls it answers), outbound (calls it makes) or both. Destroying the resource turns recording off."},
		}}
}

func (r *callRecordingPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *callRecordingPolicyModel) set(tenant int64, p client.CallRecordingPolicy) {
	m.ID = types.StringValue(fmt.Sprintf("%d/%d", tenant, p.ExtensionID))
	m.TenantID, m.ExtensionID, m.Mode = idString(tenant), idString(p.ExtensionID), types.StringValue(p.Mode)
}

func (r *callRecordingPolicyResource) write(ctx context.Context, plan *callRecordingPolicyModel, diags interface {
	AddError(string, string)
}, tenant, ext int64) bool {
	p, err := r.c.SetCallRecordingPolicy(ctx, tenant, ext, plan.Mode.ValueString())
	if err != nil {
		diags.AddError("set call recording policy", err.Error())
		return false
	}
	plan.set(tenant, p)
	return true
}

func (r *callRecordingPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m callRecordingPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.write(ctx, &m, &resp.Diagnostics, tenant, ext) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *callRecordingPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m callRecordingPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.write(ctx, &m, &resp.Diagnostics, tenant, ext) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *callRecordingPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m callRecordingPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.GetCallRecordingPolicy(ctx, tenant, ext)
	if err != nil {
		readFailed(ctx, "call recording policy", err, resp)
		return
	}
	if p.Mode == "off" { // switched off outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}
	m.set(tenant, p)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *callRecordingPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m callRecordingPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, ext := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.c.SetCallRecordingPolicy(ctx, tenant, ext, "off"); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("turn call recording off", err.Error())
	}
}

func (r *callRecordingPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tenant, ext, ok := strings.Cut(req.ID, "/")
	if !ok || tenant == "" || ext == "" {
		resp.Diagnostics.AddError("invalid import id", `expected "<tenant id>/<extension id>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), tenant)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("extension_id"), ext)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
