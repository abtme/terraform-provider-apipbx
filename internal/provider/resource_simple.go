package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

func idAttr() schema.Attribute {
	return schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
}

func replaceAttr(desc string) schema.Attribute {
	return schema.StringAttribute{Required: true, Description: desc, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}
}

// ---- apipbx_announcement

type announcementResource struct{ c *client.Client }

type announcementModel struct {
	ID          types.String `tfsdk:"id"`
	TenantID    types.String `tfsdk:"tenant_id"`
	Name        types.String `tfsdk:"name"`
	RecordingID types.String `tfsdk:"recording_id"`
	NextType    types.String `tfsdk:"next_type"`
	NextID      types.String `tfsdk:"next_id"`
}

func NewAnnouncementResource() resource.Resource { return &announcementResource{} }

func (r *announcementResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_announcement"
}

func (r *announcementResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Plays a recording, then continues to a destination. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"name":         schema.StringAttribute{Required: true},
			"recording_id": schema.StringAttribute{Required: true, Description: "Id of an apipbx_recording."},
			"next_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("hangup"),
				Description: "Where the call goes afterwards: extension, ring_group, voicemail, time_condition, ivr, queue, conference, announcement, misc_destination, set_caller_id, paging_group, voicemail_group or hangup."},
			"next_id": schema.StringAttribute{Optional: true},
		}}
}

func (r *announcementResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *announcementModel) set(a client.Announcement) {
	m.ID, m.TenantID, m.RecordingID = idString(a.ID), idString(a.TenantID), idString(a.RecordingID)
	m.Name = types.StringValue(a.Name)
	m.NextType, m.NextID = destination(a.Next)
}

func (r *announcementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m announcementModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, rec := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "recording_id", m.RecordingID)
	next := toDestination(&resp.Diagnostics, m.NextType, m.NextID)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.c.CreateAnnouncement(ctx, tenant, client.AnnouncementInput{Name: m.Name.ValueString(), RecordingID: rec, Next: next})
	if err != nil {
		resp.Diagnostics.AddError("create announcement", err.Error())
		return
	}
	m.set(a)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *announcementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m announcementModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.c.GetAnnouncement(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "announcement", err, resp)
		return
	}
	m.set(a)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *announcementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m announcementModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id, rec := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID), parseID(&resp.Diagnostics, "recording_id", m.RecordingID)
	next := toDestination(&resp.Diagnostics, m.NextType, m.NextID)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.c.UpdateAnnouncement(ctx, tenant, id, client.AnnouncementPatch{Name: ptr(m.Name.ValueString()), RecordingID: &rec, Next: next})
	if err != nil {
		resp.Diagnostics.AddError("update announcement", err.Error())
		return
	}
	m.set(a)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *announcementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m announcementModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("announcement", r.c.DeleteAnnouncement(ctx, tenant, id), resp)
}

func (r *announcementResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}

// ---- apipbx_misc_destination

type miscResource struct{ c *client.Client }

type miscModel struct {
	ID       types.String `tfsdk:"id"`
	TenantID types.String `tfsdk:"tenant_id"`
	Name     types.String `tfsdk:"name"`
	Number   types.String `tfsdk:"number"`
}

func NewMiscDestinationResource() resource.Resource { return &miscResource{} }

func (r *miscResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_misc_destination"
}

func (r *miscResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A number (internal, or external through the outbound routes) that can be a destination. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"name":   schema.StringAttribute{Required: true},
			"number": schema.StringAttribute{Required: true, Description: "2-20 characters from 0-9 * #."},
		}}
}

func (r *miscResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *miscModel) set(d client.MiscDestination) {
	m.ID, m.TenantID = idString(d.ID), idString(d.TenantID)
	m.Name, m.Number = types.StringValue(d.Name), types.StringValue(d.Number)
}

func (r *miscResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m miscModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.c.CreateMiscDestination(ctx, tenant, client.MiscDestinationInput{Name: m.Name.ValueString(), Number: m.Number.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create misc destination", err.Error())
		return
	}
	m.set(d)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *miscResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m miscModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.c.GetMiscDestination(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "misc destination", err, resp)
		return
	}
	m.set(d)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *miscResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m miscModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.c.UpdateMiscDestination(ctx, tenant, id, client.MiscDestinationPatch{Name: ptr(m.Name.ValueString()), Number: ptr(m.Number.ValueString())})
	if err != nil {
		resp.Diagnostics.AddError("update misc destination", err.Error())
		return
	}
	m.set(d)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *miscResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m miscModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("misc destination", r.c.DeleteMiscDestination(ctx, tenant, id), resp)
}

func (r *miscResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}

// ---- apipbx_caller_id_step

type callerIDResource struct{ c *client.Client }

type callerIDModel struct {
	ID         types.String `tfsdk:"id"`
	TenantID   types.String `tfsdk:"tenant_id"`
	Name       types.String `tfsdk:"name"`
	NamePrefix types.String `tfsdk:"name_prefix"`
	Number     types.String `tfsdk:"number"`
	NextType   types.String `tfsdk:"next_type"`
	NextID     types.String `tfsdk:"next_id"`
}

func NewCallerIDStepResource() resource.Resource { return &callerIDResource{} }

func (r *callerIDResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_caller_id_step"
}

func (r *callerIDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Changes the caller id of a call, then continues to a destination. Set a name_prefix, a number, or both. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"name":        schema.StringAttribute{Required: true},
			"name_prefix": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Put before the caller's name (up to 20 characters)."},
			"number":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Replaces the caller's number (digits, optional leading +)."},
			"next_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("hangup"),
				Description: "Where the call goes afterwards: extension, ring_group, voicemail, time_condition, ivr, queue, conference, announcement, misc_destination, set_caller_id, paging_group, voicemail_group or hangup."},
			"next_id": schema.StringAttribute{Optional: true},
		}}
}

func (r *callerIDResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *callerIDModel) set(c client.CallerIDStep) {
	m.ID, m.TenantID = idString(c.ID), idString(c.TenantID)
	m.Name, m.NamePrefix, m.Number = types.StringValue(c.Name), types.StringValue(c.NamePrefix), types.StringValue(c.Number)
	m.NextType, m.NextID = destination(c.Next)
}

func (r *callerIDResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m callerIDModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	next := toDestination(&resp.Diagnostics, m.NextType, m.NextID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.CreateCallerIDStep(ctx, tenant, client.CallerIDStepInput{Name: m.Name.ValueString(), NamePrefix: m.NamePrefix.ValueString(), Number: m.Number.ValueString(), Next: next})
	if err != nil {
		resp.Diagnostics.AddError("create caller id step", err.Error())
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *callerIDResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m callerIDModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.GetCallerIDStep(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "caller id step", err, resp)
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *callerIDResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m callerIDModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	next := toDestination(&resp.Diagnostics, m.NextType, m.NextID)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.UpdateCallerIDStep(ctx, tenant, id, client.CallerIDStepPatch{Name: ptr(m.Name.ValueString()), NamePrefix: ptr(m.NamePrefix.ValueString()),
		Number: ptr(m.Number.ValueString()), Next: next})
	if err != nil {
		resp.Diagnostics.AddError("update caller id step", err.Error())
		return
	}
	m.set(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *callerIDResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m callerIDModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("caller id step", r.c.DeleteCallerIDStep(ctx, tenant, id), resp)
}

func (r *callerIDResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}

// ---- apipbx_speed_dial

type speedDialResource struct{ c *client.Client }

type speedDialModel struct {
	ID          types.String `tfsdk:"id"`
	TenantID    types.String `tfsdk:"tenant_id"`
	ExtensionID types.String `tfsdk:"extension_id"`
	Code        types.String `tfsdk:"code"`
	Number      types.String `tfsdk:"number"`
	Description types.String `tfsdk:"description"`
}

func NewSpeedDialResource() resource.Resource { return &speedDialResource{} }

func (r *speedDialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_speed_dial"
}

func (r *speedDialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A speed dial: extensions dial *0 and the code to call the number. Import with \"<tenant id>/<id>\".",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(), "tenant_id": replaceAttr(""),
			"extension_id": schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "Make it this extension's own speed dial, which wins over the tenant's of the same code. Without it the speed dial is the tenant's."},
			"code":        replaceAttr("1-4 digits."),
			"number":      schema.StringAttribute{Required: true, Description: "2-20 characters from 0-9 * #: internal, or external through the outbound routes."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
		}}
}

func (r *speedDialResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *speedDialModel) set(s client.SpeedDial) {
	m.ID, m.TenantID = idString(s.ID), idString(s.TenantID)
	m.ExtensionID = types.StringNull()
	if s.ExtensionID != nil {
		m.ExtensionID = idString(*s.ExtensionID)
	}
	m.Code, m.Number, m.Description = types.StringValue(s.Code), types.StringValue(s.Number), types.StringValue(s.Description)
}

func (r *speedDialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m speedDialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant := parseID(&resp.Diagnostics, "tenant_id", m.TenantID)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.SpeedDialInput{Code: m.Code.ValueString(), Number: m.Number.ValueString(), Description: m.Description.ValueString()}
	if !m.ExtensionID.IsNull() && !m.ExtensionID.IsUnknown() {
		ext := parseID(&resp.Diagnostics, "extension_id", m.ExtensionID)
		if resp.Diagnostics.HasError() {
			return
		}
		in.ExtensionID = &ext
	}
	s, err := r.c.CreateSpeedDial(ctx, tenant, in)
	if err != nil {
		resp.Diagnostics.AddError("create speed dial", err.Error())
		return
	}
	m.set(s)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *speedDialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m speedDialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.c.GetSpeedDial(ctx, tenant, id)
	if err != nil {
		readFailed(ctx, "speed dial", err, resp)
		return
	}
	m.set(s)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *speedDialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m speedDialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.c.UpdateSpeedDial(ctx, tenant, id, client.SpeedDialPatch{Number: ptr(m.Number.ValueString()), Description: ptr(m.Description.ValueString())})
	if err != nil {
		resp.Diagnostics.AddError("update speed dial", err.Error())
		return
	}
	m.set(s)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *speedDialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m speedDialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	tenant, id := parseID(&resp.Diagnostics, "tenant_id", m.TenantID), parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("speed dial", r.c.DeleteSpeedDial(ctx, tenant, id), resp)
}

func (r *speedDialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTenantScoped(ctx, "id", req, resp)
}
