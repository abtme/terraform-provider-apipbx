package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

// apipbx_backup_settings: how the server backs itself up (when, how many, where, and an optional SFTP copy on another machine), a singleton
// (needs a platform key). Only the attributes you set are sent. With the remote enabled the server's backup key is made if there is none and
// its public half is returned as public_key, to be put in the remote user's authorized_keys.

type backupSettingsResource struct{ c *client.Client }

type backupSettingsModel struct {
	ID            types.String `tfsdk:"id"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Time          types.String `tfsdk:"time"`
	Keep          types.Int64  `tfsdk:"keep"`
	Dir           types.String `tfsdk:"dir"`
	Recordings    types.Bool   `tfsdk:"recordings"`
	RemoteEnabled types.Bool   `tfsdk:"remote_enabled"`
	RemoteHost    types.String `tfsdk:"remote_host"`
	RemotePort    types.Int64  `tfsdk:"remote_port"`
	RemoteUser    types.String `tfsdk:"remote_user"`
	RemotePath    types.String `tfsdk:"remote_path"`
	RemoteKeep    types.Int64  `tfsdk:"remote_keep"`
	PublicKey     types.String `tfsdk:"public_key"`
}

func NewBackupSettingsResource() resource.Resource { return &backupSettingsResource{} }

func (r *backupSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_settings"
}

func (r *backupSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Description: desc, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
	}
	num := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Optional: true, Computed: true, Description: desc, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}
	}
	flag := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Optional: true, Computed: true, Description: desc, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}
	}
	resp.Schema = schema.Schema{
		Description: "How the server backs itself up: a singleton, needs a platform key. apipbxd makes the backups itself (a database dump, and the call recordings if " +
			"included) at the time set, keeps the newest `keep` in `dir`, and can send each to another machine over SFTP. Only the attributes you set are sent. Destroying " +
			"this resource leaves the settings as they are. Import with \"backup\".",
		Attributes: map[string]schema.Attribute{
			"id":             idAttr(),
			"enabled":        flag("Make a backup every day (default true)."),
			"time":           str("The time of day (server time, HH:MM) the daily backup is made at (default 03:00). One missed because the server was off is made when it is back."),
			"keep":           num("How many backups stay on the server; the oldest are removed; 0 keeps all (default 14)."),
			"dir":            str("The folder the backups go to (default /var/backups/apipbx). The service can write only under /var/backups/apipbx and /var/lib/apipbx: another folder must be allowed on the server first."),
			"recordings":     flag("Include the call recordings, which are files and can be large (default true)."),
			"remote_enabled": flag("Also copy each backup to another machine over SFTP."),
			"remote_host":    str("The remote machine's host name or address."),
			"remote_port":    num("The remote SSH port (default 22)."),
			"remote_user":    str("The user on the remote machine."),
			"remote_path":    str("The folder on the remote machine (made if it is missing)."),
			"remote_keep":    num("How many backups stay on the remote machine; 0 keeps all (default 14)."),
			"public_key":     schema.StringAttribute{Computed: true, Description: "The server's backup public key, made when the remote is enabled: add it to the remote user's ~/.ssh/authorized_keys."},
		}}
}

func (r *backupSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

type nullable interface {
	IsNull() bool
	IsUnknown() bool
}

// body is what the plan sets: the attributes that are configured, by their API names (the remote's nested).
func (m backupSettingsModel) body() map[string]any {
	known := func(v nullable) bool { return !v.IsNull() && !v.IsUnknown() }
	b, rem := map[string]any{}, map[string]any{}
	if known(m.Enabled) {
		b["enabled"] = m.Enabled.ValueBool()
	}
	if known(m.Time) {
		b["time"] = m.Time.ValueString()
	}
	if known(m.Keep) {
		b["keep"] = m.Keep.ValueInt64()
	}
	if known(m.Dir) {
		b["dir"] = m.Dir.ValueString()
	}
	if known(m.Recordings) {
		b["recordings"] = m.Recordings.ValueBool()
	}
	if known(m.RemoteEnabled) {
		rem["enabled"] = m.RemoteEnabled.ValueBool()
	}
	if known(m.RemoteHost) {
		rem["host"] = m.RemoteHost.ValueString()
	}
	if known(m.RemotePort) {
		rem["port"] = m.RemotePort.ValueInt64()
	}
	if known(m.RemoteUser) {
		rem["user"] = m.RemoteUser.ValueString()
	}
	if known(m.RemotePath) {
		rem["path"] = m.RemotePath.ValueString()
	}
	if known(m.RemoteKeep) {
		rem["keep"] = m.RemoteKeep.ValueInt64()
	}
	if len(rem) > 0 {
		b["remote"] = rem
	}
	return b
}

func (m *backupSettingsModel) set(s client.BackupStatus) {
	c := s.Config
	m.ID = types.StringValue("backup")
	m.Enabled = types.BoolValue(c.Enabled)
	m.Time = types.StringValue(c.Time)
	m.Keep = types.Int64Value(int64(c.Keep))
	m.Dir = types.StringValue(c.Dir)
	m.Recordings = types.BoolValue(c.Recordings)
	m.RemoteEnabled = types.BoolValue(c.Remote.Enabled)
	m.RemoteHost = types.StringValue(c.Remote.Host)
	m.RemotePort = types.Int64Value(int64(c.Remote.Port))
	m.RemoteUser = types.StringValue(c.Remote.User)
	m.RemotePath = types.StringValue(c.Remote.Path)
	m.RemoteKeep = types.Int64Value(int64(c.Remote.Keep))
	m.PublicKey = types.StringValue(s.PublicKey)
}

func (r *backupSettingsResource) write(ctx context.Context, m *backupSettingsModel) error {
	st, err := r.c.UpdateBackup(ctx, m.body())
	if err != nil {
		return err
	}
	if st.Config.Remote.Enabled && st.PublicKey == "" {
		if st.PublicKey, err = r.c.MakeBackupKey(ctx); err != nil {
			return err
		}
	}
	m.set(st)
	return nil
}

func (r *backupSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m backupSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &m); err != nil {
		resp.Diagnostics.AddError("set the backup settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *backupSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m backupSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &m); err != nil {
		resp.Diagnostics.AddError("update the backup settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *backupSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m backupSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	st, err := r.c.GetBackup(ctx)
	if err != nil {
		readFailed(ctx, "backup settings", err, resp)
		return
	}
	m.set(st)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *backupSettingsResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	// the settings belong to the server: destroying the resource only stops managing them
}

func (r *backupSettingsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "backup")...)
}
