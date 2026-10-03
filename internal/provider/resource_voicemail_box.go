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

type voicemailBoxResource struct{ c *client.Client }

type voicemailBoxModel struct {
	ID               types.String `tfsdk:"id"`
	TenantID         types.String `tfsdk:"tenant_id"`
	Number           types.String `tfsdk:"number"`
	Name             types.String `tfsdk:"name"`
	PIN              types.String `tfsdk:"pin"`
	Email            types.String `tfsdk:"email"`
	Attach           types.Bool   `tfsdk:"attach"`
	DeleteAfterEmail types.Bool   `tfsdk:"delete_after_email"`
	SayCID           types.Bool   `tfsdk:"say_cid"`
	Envelope         types.Bool   `tfsdk:"envelope"`
	MaxMessages      types.Int64  `tfsdk:"max_messages"`
}

func NewVoicemailBoxResource() resource.Resource { return &voicemailBoxResource{} }

func (r *voicemailBoxResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_voicemail_box"
}

func (r *voicemailBoxResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A voicemail box. Deleting it deletes its messages. Import with \"<tenant id>/<box id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"number":    schema.StringAttribute{Required: true, Description: "2-16 digits.", PlanModifiers: replace},
			"name":      schema.StringAttribute{Required: true},
			"pin": schema.StringAttribute{Optional: true, Computed: true, Sensitive: true,
				Description:   "3-10 digits; generated when omitted. Leave it out if the owner sets it from the phone.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"email":              schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Notification addresses, separated by spaces or commas."},
			"attach":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Attach the recording to the email."},
			"delete_after_email": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"say_cid":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"envelope":           schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"max_messages":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(100)},
		}}
}

func (r *voicemailBoxResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *voicemailBoxModel) set(b client.VoicemailBox) {
	m.ID, m.TenantID = idString(b.ID), idString(b.TenantID)
	m.Number, m.Name, m.PIN, m.Email = types.StringValue(b.Number), types.StringValue(b.Name), types.StringValue(b.PIN), types.StringValue(b.Email)
	m.Attach, m.DeleteAfterEmail = types.BoolValue(b.Attach), types.BoolValue(b.DeleteAfterEmail)
	m.SayCID, m.Envelope, m.MaxMessages = types.BoolValue(b.SayCID), types.BoolValue(b.Envelope), types.Int64Value(int64(b.MaxMessages))
}

func (r *voicemailBoxResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m voicemailBoxModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.c.CreateVoicemailBox(ctx, tenant, client.VoicemailBoxInput{
		Number: m.Number.ValueString(), Name: m.Name.ValueString(), PIN: m.PIN.ValueString(), Email: m.Email.ValueString(),
		Attach: m.Attach.ValueBool(), DeleteAfterEmail: m.DeleteAfterEmail.ValueBool(), SayCID: m.SayCID.ValueBool(),
		Envelope: m.Envelope.ValueBool(), MaxMessages: int(m.MaxMessages.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError("create voicemail box", err.Error())
		return
	}
	m.set(b)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *voicemailBoxResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m voicemailBoxModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.c.GetVoicemailBox(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "voicemail box", err, resp)
		return
	}
	m.set(b)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *voicemailBoxResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m voicemailBoxModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.c.UpdateVoicemailBox(ctx, tenant, id, client.VoicemailBoxPatch{
		Name: ptr(m.Name.ValueString()), PIN: ptr(m.PIN.ValueString()), Email: ptr(m.Email.ValueString()),
		Attach: ptr(m.Attach.ValueBool()), DeleteAfterEmail: ptr(m.DeleteAfterEmail.ValueBool()),
		SayCID: ptr(m.SayCID.ValueBool()), Envelope: ptr(m.Envelope.ValueBool()), MaxMessages: ptr(int(m.MaxMessages.ValueInt64())),
	})
	if err != nil {
		resp.Diagnostics.AddError("update voicemail box", err.Error())
		return
	}
	m.set(b)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *voicemailBoxResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m voicemailBoxModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("voicemail box", r.c.DeleteVoicemailBox(ctx, tenant, id), resp)
}

func (r *voicemailBoxResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
