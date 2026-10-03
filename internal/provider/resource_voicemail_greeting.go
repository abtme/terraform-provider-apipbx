package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type voicemailGreetingResource struct{ c *client.Client }

type voicemailGreetingModel struct {
	ID              types.String  `tfsdk:"id"`
	TenantID        types.String  `tfsdk:"tenant_id"`
	VoicemailBoxID  types.String  `tfsdk:"voicemail_box_id"`
	Type            types.String  `tfsdk:"type"`
	ContentBase64   types.String  `tfsdk:"content_base64"`
	DurationSeconds types.Float64 `tfsdk:"duration_seconds"`
}

func NewVoicemailGreetingResource() resource.Resource { return &voicemailGreetingResource{} }

func (r *voicemailGreetingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_voicemail_greeting"
}

func (r *voicemailGreetingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A voicemail greeting. Import with \"<tenant id>/<box id>/<type>\" (the audio is then replaced by the configured file on the next apply).",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id":        schema.StringAttribute{Required: true, PlanModifiers: replace},
			"voicemail_box_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"type": schema.StringAttribute{Required: true, PlanModifiers: replace,
				Description: "unavailable or busy (replace the default prompts), name (the spoken name) or temporary (overrides the others)."},
			"content_base64": schema.StringAttribute{Required: true,
				Description: "The WAV file, base64 encoded: filebase64(\"away.wav\"). 16-bit PCM, mono, 8000 Hz."},
			"duration_seconds": schema.Float64Attribute{Computed: true},
		}}
}

func (r *voicemailGreetingResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (r *voicemailGreetingResource) apply(ctx context.Context, m *voicemailGreetingModel, diags interface {
	AddError(string, string)
}, id func() (int64, int64)) bool {
	tenant, box := id()
	audio, err := base64.StdEncoding.DecodeString(m.ContentBase64.ValueString())
	if err != nil {
		diags.AddError("invalid content_base64", err.Error())
		return false
	}
	g, err := r.c.SetVoicemailGreeting(ctx, tenant, box, m.Type.ValueString(), audio)
	if err != nil {
		diags.AddError("set voicemail greeting", err.Error())
		return false
	}
	m.ID = types.StringValue(fmt.Sprintf("%d/%d/%s", tenant, box, m.Type.ValueString()))
	m.DurationSeconds = types.Float64Value(g.DurationSeconds)
	return true
}

func (r *voicemailGreetingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m voicemailGreetingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, box := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "voicemail_box_id", m.VoicemailBoxID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, &resp.Diagnostics, func() (int64, int64) { return tenant, box }) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *voicemailGreetingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m voicemailGreetingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, box := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "voicemail_box_id", m.VoicemailBoxID)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.apply(ctx, &m, &resp.Diagnostics, func() (int64, int64) { return tenant, box }) {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *voicemailGreetingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m voicemailGreetingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, box := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "voicemail_box_id", m.VoicemailBoxID)
	if resp.Diagnostics.HasError() {
		return
	}
	audio, err := r.c.GetVoicemailGreetingAudio(ctx, tenant, box, m.Type.ValueString())
	if err != nil {
		readFailed(ctx, "voicemail greeting", err, resp)
		return
	}
	// A different file (recorded from the phone, or replaced through the API) shows up as a change.
	if want, derr := base64.StdEncoding.DecodeString(m.ContentBase64.ValueString()); derr != nil || sha256.Sum256(want) != sha256.Sum256(audio) {
		m.ContentBase64 = types.StringValue("")
	}
	if g, err := r.c.GetVoicemailGreeting(ctx, tenant, box, m.Type.ValueString()); err == nil {
		m.DurationSeconds = types.Float64Value(g.DurationSeconds)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *voicemailGreetingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m voicemailGreetingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, box := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "voicemail_box_id", m.VoicemailBoxID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("voicemail greeting", r.c.DeleteVoicemailGreeting(ctx, tenant, box, m.Type.ValueString()), resp)
}

func (r *voicemailGreetingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("invalid import id", `expected "<tenant id>/<box id>/<type>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("voicemail_box_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
