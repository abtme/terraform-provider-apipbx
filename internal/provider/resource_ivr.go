package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type ivrResource struct{ c *client.Client }

type ivrEntryModel struct {
	Digit           types.String `tfsdk:"digit"`
	DestinationType types.String `tfsdk:"destination_type"`
	DestinationID   types.String `tfsdk:"destination_id"`
}

type ivrModel struct {
	ID             types.String    `tfsdk:"id"`
	TenantID       types.String    `tfsdk:"tenant_id"`
	Name           types.String    `tfsdk:"name"`
	Announcement   types.String    `tfsdk:"announcement"`
	RecordingID    types.String    `tfsdk:"recording_id"`
	TimeoutSeconds types.Int64     `tfsdk:"timeout_seconds"`
	MaxRetries     types.Int64     `tfsdk:"max_retries"`
	Entries        []ivrEntryModel `tfsdk:"entries"`
	TimeoutType    types.String    `tfsdk:"timeout_type"`
	TimeoutID      types.String    `tfsdk:"timeout_id"`
	InvalidType    types.String    `tfsdk:"invalid_type"`
	InvalidID      types.String    `tfsdk:"invalid_id"`
}

func NewIVRResource() resource.Resource { return &ivrResource{} }

func (r *ivrResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ivr"
}

func (r *ivrResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	dest := "extension, ring_group, voicemail, time_condition, ivr, queue, conference, announcement, misc_destination, set_caller_id, paging_group, voicemail_group, disa or hangup."
	resp.Schema = schema.Schema{
		Description: "An IVR menu. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":      schema.StringAttribute{Required: true},
			"announcement": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Description: "Asterisk sound name(s) joined with &, such as custom/welcome; empty plays nothing. Sounds are server-wide."},
			"recording_id":    schema.StringAttribute{Optional: true, Description: "Id of an apipbx_recording to play instead of the announcement."},
			"timeout_seconds": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(10), Description: "Seconds to wait for a key (1-60)."},
			"max_retries":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(3), Description: "Unknown keys, and silences, allowed before giving up (1-10)."},
			"entries": schema.ListNestedAttribute{Optional: true, Computed: true, Description: "Keys of the menu (at most 12).",
				Default: listdefault.StaticValue(types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
					"digit": types.StringType, "destination_type": types.StringType, "destination_id": types.StringType}}, []attr.Value{})),
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"digit":            schema.StringAttribute{Required: true, Description: "One of 0-9 or *."},
					"destination_type": schema.StringAttribute{Required: true, Description: dest},
					"destination_id":   schema.StringAttribute{Optional: true, Description: "Id of that destination (not for hangup)."},
				}}},
			"timeout_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("hangup"), Description: "Where silence leads after the retries: " + dest},
			"timeout_id":   schema.StringAttribute{Optional: true},
			"invalid_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("hangup"), Description: "Where unknown keys lead after the retries: " + dest},
			"invalid_id":   schema.StringAttribute{Optional: true},
		}}
}

func (r *ivrResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *ivrModel) set(v client.IVR) {
	m.ID, m.TenantID = idString(v.ID), idString(v.TenantID)
	m.Name, m.Announcement = types.StringValue(v.Name), types.StringValue(v.Announcement)
	m.RecordingID = types.StringNull()
	if v.RecordingID != nil {
		m.RecordingID = idString(*v.RecordingID)
	}
	m.TimeoutSeconds, m.MaxRetries = types.Int64Value(int64(v.TimeoutSeconds)), types.Int64Value(int64(v.MaxRetries))
	m.Entries = make([]ivrEntryModel, 0, len(v.Entries))
	for _, e := range v.Entries {
		t, id := destination(e.Destination)
		m.Entries = append(m.Entries, ivrEntryModel{types.StringValue(e.Digit), t, id})
	}
	m.TimeoutType, m.TimeoutID = destination(v.TimeoutDestination)
	m.InvalidType, m.InvalidID = destination(v.InvalidDestination)
}

// entries converts the configured keys; an unset list is an empty menu.
func (m ivrModel) entries(d *diag.Diagnostics) []client.IVREntry {
	out := make([]client.IVREntry, 0, len(m.Entries))
	for _, e := range m.Entries {
		out = append(out, client.IVREntry{Digit: e.Digit.ValueString(), Destination: *toDestination(d, e.DestinationType, e.DestinationID)})
	}
	return out
}

func (r *ivrResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m ivrModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	entries := m.entries(&resp.Diagnostics)
	to := toDestination(&resp.Diagnostics, m.TimeoutType, m.TimeoutID)
	inv := toDestination(&resp.Diagnostics, m.InvalidType, m.InvalidID)
	var rec *int64
	if !m.RecordingID.IsNull() {
		rec = ptr(parseID(&resp.Diagnostics, "recording_id", m.RecordingID))
	}
	if resp.Diagnostics.HasError() {
		return
	}
	v, err := r.c.CreateIVR(ctx, tenant, client.IVRInput{Name: m.Name.ValueString(), Announcement: m.Announcement.ValueString(), RecordingID: rec,
		TimeoutSeconds: int(m.TimeoutSeconds.ValueInt64()), MaxRetries: int(m.MaxRetries.ValueInt64()),
		Entries: entries, TimeoutDestination: to, InvalidDestination: inv})
	if err != nil {
		resp.Diagnostics.AddError("create ivr", err.Error())
		return
	}
	m.set(v)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *ivrResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m ivrModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	v, err := r.c.GetIVR(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "ivr", err, resp)
		return
	}
	m.set(v)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *ivrResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m ivrModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	entries := m.entries(&resp.Diagnostics)
	to := toDestination(&resp.Diagnostics, m.TimeoutType, m.TimeoutID)
	inv := toDestination(&resp.Diagnostics, m.InvalidType, m.InvalidID)
	rec := ptr(int64(0)) // 0 removes the recording
	if !m.RecordingID.IsNull() {
		rec = ptr(parseID(&resp.Diagnostics, "recording_id", m.RecordingID))
	}
	if resp.Diagnostics.HasError() {
		return
	}
	v, err := r.c.UpdateIVR(ctx, tenant, id, client.IVRPatch{Name: ptr(m.Name.ValueString()), Announcement: ptr(m.Announcement.ValueString()), RecordingID: rec,
		TimeoutSeconds: ptr(int(m.TimeoutSeconds.ValueInt64())), MaxRetries: ptr(int(m.MaxRetries.ValueInt64())),
		Entries: &entries, TimeoutDestination: to, InvalidDestination: inv})
	if err != nil {
		resp.Diagnostics.AddError("update ivr", err.Error())
		return
	}
	m.set(v)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *ivrResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m ivrModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("ivr", r.c.DeleteIVR(ctx, tenant, id), resp)
}

func (r *ivrResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
