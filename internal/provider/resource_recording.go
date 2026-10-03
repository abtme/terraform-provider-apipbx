package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type recordingResource struct{ c *client.Client }

type recordingModel struct {
	ID              types.String  `tfsdk:"id"`
	TenantID        types.String  `tfsdk:"tenant_id"`
	Name            types.String  `tfsdk:"name"`
	Description     types.String  `tfsdk:"description"`
	ContentBase64   types.String  `tfsdk:"content_base64"`
	DurationSeconds types.Float64 `tfsdk:"duration_seconds"`
	SHA256          types.String  `tfsdk:"sha256"`
}

func NewRecordingResource() resource.Resource { return &recordingResource{} }

func (r *recordingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recording"
}

func (r *recordingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A stored announcement. The audio must be a WAV file of 16-bit PCM, mono, 8000 Hz. Import is not supported: the audio cannot be read back into the configuration.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id":   schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":        schema.StringAttribute{Required: true},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"content_base64": schema.StringAttribute{Required: true,
				Description: "The WAV file, base64 encoded: filebase64(\"welcome.wav\"). Convert other audio with ffmpeg -i in.mp3 -ar 8000 -ac 1 -c:a pcm_s16le out.wav."},
			"duration_seconds": schema.Float64Attribute{Computed: true},
			"sha256":           schema.StringAttribute{Computed: true, Description: "SHA-256 of the stored WAV file."},
		}}
}

func (r *recordingResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *recordingModel) set(rec client.Recording) {
	m.ID, m.TenantID = idString(rec.ID), idString(rec.TenantID)
	m.Name, m.Description = types.StringValue(rec.Name), types.StringValue(rec.Description)
	m.DurationSeconds, m.SHA256 = types.Float64Value(rec.DurationSeconds), types.StringValue(rec.SHA256)
}

func decodeAudio(m recordingModel, resp interface{ AddError(string, string) }) ([]byte, bool) {
	b, err := base64.StdEncoding.DecodeString(m.ContentBase64.ValueString())
	if err != nil {
		resp.AddError("invalid content_base64", err.Error())
		return nil, false
	}
	return b, true
}

func (r *recordingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m recordingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	audio, ok := decodeAudio(m, &resp.Diagnostics)
	if !ok || resp.Diagnostics.HasError() {
		return
	}
	rec, err := r.c.CreateRecording(ctx, tenant, client.RecordingInput{Name: m.Name.ValueString(), Description: m.Description.ValueString(), Audio: audio})
	if err != nil {
		resp.Diagnostics.AddError("create recording", err.Error())
		return
	}
	m.set(rec)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *recordingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m recordingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	rec, err := r.c.GetRecording(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "recording", err, resp)
		return
	}
	// The audio itself is not read back; a changed checksum means it was replaced outside Terraform.
	if audio, err := base64.StdEncoding.DecodeString(m.ContentBase64.ValueString()); err == nil {
		sum := sha256.Sum256(audio)
		if hex.EncodeToString(sum[:]) != rec.SHA256 {
			m.ContentBase64 = types.StringValue("")
		}
	}
	m.set(rec)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *recordingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m recordingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	audio, ok := decodeAudio(m, &resp.Diagnostics)
	if !ok || resp.Diagnostics.HasError() {
		return
	}
	rec, err := r.c.UpdateRecording(ctx, tenant, id, client.RecordingPatch{Name: ptr(m.Name.ValueString()),
		Description: ptr(m.Description.ValueString()), Audio: &audio})
	if err != nil {
		resp.Diagnostics.AddError("update recording", err.Error())
		return
	}
	m.set(rec)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *recordingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m recordingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("recording", r.c.DeleteRecording(ctx, tenant, id), resp)
}
