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

// apipbx_server_settings: the server's own settings, a singleton (needs a platform key). Only the attributes you set are sent; the others
// stay as the server has them and are read back.

type serverSettingsResource struct{ c *client.Client }

type serverSettingsModel struct {
	ID            types.String `tfsdk:"id"`
	AMISecret     types.String `tfsdk:"ami_secret"`
	ODBCDSN       types.String `tfsdk:"odbc_dsn"`
	DBUser        types.String `tfsdk:"db_user"`
	AMIUser       types.String `tfsdk:"ami_user"`
	AMIBind       types.String `tfsdk:"ami_bind"`
	AMIPort       types.Int64  `tfsdk:"ami_port"`
	SIPPort       types.Int64  `tfsdk:"sip_port"`
	IAXPort       types.Int64  `tfsdk:"iax_port"`
	ExternalIP    types.String `tfsdk:"external_ip"`
	LocalNet      types.String `tfsdk:"local_net"`
	RTPStart      types.Int64  `tfsdk:"rtp_start"`
	RTPEnd        types.Int64  `tfsdk:"rtp_end"`
	VoicemailFrom types.String `tfsdk:"voicemail_from"`
	MailCmd       types.String `tfsdk:"mail_cmd"`
	MediaURL      types.String `tfsdk:"media_url"`
	RecordingsDir types.String `tfsdk:"call_recordings_dir"`
	TLSCertFile   types.String `tfsdk:"tls_cert"`
	TLSKeyFile    types.String `tfsdk:"tls_key"`
	TLSPort       types.Int64  `tfsdk:"tls_port"`
	WSSPort       types.Int64  `tfsdk:"wss_port"`
	TLSOnly       types.Bool   `tfsdk:"tls_only"`
}

func NewServerSettingsResource() resource.Resource { return &serverSettingsResource{} }

func (r *serverSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_settings"
}

func (r *serverSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The server's own settings, kept in its database (so a backup holds them) and applied to Asterisk's configuration files: a singleton, needs a " +
			"platform key. Only the attributes you set are sent; the others keep the server's value and are read back. A change to a port, TLS or the manager login " +
			"restarts Asterisk and apipbxd. Destroying this resource leaves the settings as they are. Import with \"settings\".",
		Attributes: map[string]schema.Attribute{
			"id":                  idAttr(),
			"ami_secret":          schema.StringAttribute{Optional: true, Sensitive: true, Description: "The manager secret (8-128 characters, no spaces or ; = \\). Write-only: the server never returns it, so a change made elsewhere is not detected."},
			"odbc_dsn":            schema.StringAttribute{Optional: true, Computed: true, Description: "The ODBC data source Asterisk reads the database through (default apipbx).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"db_user":             schema.StringAttribute{Optional: true, Computed: true, Description: "The database user Asterisk connects as.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"ami_user":            schema.StringAttribute{Optional: true, Computed: true, Description: "The Asterisk manager login apipbxd uses to reload Asterisk. Changing it restarts Asterisk and apipbxd.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"ami_bind":            schema.StringAttribute{Optional: true, Computed: true, Description: "The manager's listen address.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"ami_port":            schema.Int64Attribute{Optional: true, Computed: true, Description: "The manager's listen port.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"sip_port":            schema.Int64Attribute{Optional: true, Computed: true, Description: "The SIP UDP port.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"iax_port":            schema.Int64Attribute{Optional: true, Computed: true, Description: "The IAX2 UDP port.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"external_ip":         schema.StringAttribute{Optional: true, Computed: true, Description: "The public IP when behind NAT; empty = none.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"local_net":           schema.StringAttribute{Optional: true, Computed: true, Description: "The local network CIDR, with external_ip.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"rtp_start":           schema.Int64Attribute{Optional: true, Computed: true, Description: "The first audio (RTP) port.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"rtp_end":             schema.Int64Attribute{Optional: true, Computed: true, Description: "The last audio (RTP) port.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"voicemail_from":      schema.StringAttribute{Optional: true, Computed: true, Description: "The From address of voicemail emails.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"mail_cmd":            schema.StringAttribute{Optional: true, Computed: true, Description: "The command voicemail emails are piped to.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"media_url":           schema.StringAttribute{Optional: true, Computed: true, Description: "Where Asterisk fetches recordings from (apipbxd's /media).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"call_recordings_dir": schema.StringAttribute{Optional: true, Computed: true, Description: "Where call recordings are written.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tls_cert":            schema.StringAttribute{Optional: true, Computed: true, Description: "The TLS certificate file (PEM, full chain) on the server; with tls_key it switches on SIP over TLS and WebRTC.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tls_key":             schema.StringAttribute{Optional: true, Computed: true, Description: "The TLS key file (PEM) on the server.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tls_port":            schema.Int64Attribute{Optional: true, Computed: true, Description: "The SIP over TLS port.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"wss_port":            schema.Int64Attribute{Optional: true, Computed: true, Description: "The secure WebSocket (WebRTC) port; -1 leaves WebRTC off.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"tls_only":            schema.BoolAttribute{Optional: true, Computed: true, Description: "Accept no plain SIP at all (needs a certificate).", PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		}}
}

func (r *serverSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

// body is what the plan sets: the attributes that are configured, by their API names.
func (m serverSettingsModel) body() map[string]any {
	b := map[string]any{}
	if !m.AMISecret.IsNull() && !m.AMISecret.IsUnknown() {
		b["ami_secret"] = m.AMISecret.ValueString()
	}
	if !m.ODBCDSN.IsNull() && !m.ODBCDSN.IsUnknown() {
		b["odbc_dsn"] = m.ODBCDSN.ValueString()
	}
	if !m.DBUser.IsNull() && !m.DBUser.IsUnknown() {
		b["db_user"] = m.DBUser.ValueString()
	}
	if !m.AMIUser.IsNull() && !m.AMIUser.IsUnknown() {
		b["ami_user"] = m.AMIUser.ValueString()
	}
	if !m.AMIBind.IsNull() && !m.AMIBind.IsUnknown() {
		b["ami_bind"] = m.AMIBind.ValueString()
	}
	if !m.AMIPort.IsNull() && !m.AMIPort.IsUnknown() {
		b["ami_port"] = m.AMIPort.ValueInt64()
	}
	if !m.SIPPort.IsNull() && !m.SIPPort.IsUnknown() {
		b["sip_port"] = m.SIPPort.ValueInt64()
	}
	if !m.IAXPort.IsNull() && !m.IAXPort.IsUnknown() {
		b["iax_port"] = m.IAXPort.ValueInt64()
	}
	if !m.ExternalIP.IsNull() && !m.ExternalIP.IsUnknown() {
		b["external_ip"] = m.ExternalIP.ValueString()
	}
	if !m.LocalNet.IsNull() && !m.LocalNet.IsUnknown() {
		b["local_net"] = m.LocalNet.ValueString()
	}
	if !m.RTPStart.IsNull() && !m.RTPStart.IsUnknown() {
		b["rtp_start"] = m.RTPStart.ValueInt64()
	}
	if !m.RTPEnd.IsNull() && !m.RTPEnd.IsUnknown() {
		b["rtp_end"] = m.RTPEnd.ValueInt64()
	}
	if !m.VoicemailFrom.IsNull() && !m.VoicemailFrom.IsUnknown() {
		b["voicemail_from"] = m.VoicemailFrom.ValueString()
	}
	if !m.MailCmd.IsNull() && !m.MailCmd.IsUnknown() {
		b["mail_cmd"] = m.MailCmd.ValueString()
	}
	if !m.MediaURL.IsNull() && !m.MediaURL.IsUnknown() {
		b["media_url"] = m.MediaURL.ValueString()
	}
	if !m.RecordingsDir.IsNull() && !m.RecordingsDir.IsUnknown() {
		b["call_recordings_dir"] = m.RecordingsDir.ValueString()
	}
	if !m.TLSCertFile.IsNull() && !m.TLSCertFile.IsUnknown() {
		b["tls_cert"] = m.TLSCertFile.ValueString()
	}
	if !m.TLSKeyFile.IsNull() && !m.TLSKeyFile.IsUnknown() {
		b["tls_key"] = m.TLSKeyFile.ValueString()
	}
	if !m.TLSPort.IsNull() && !m.TLSPort.IsUnknown() {
		b["tls_port"] = m.TLSPort.ValueInt64()
	}
	if !m.WSSPort.IsNull() && !m.WSSPort.IsUnknown() {
		b["wss_port"] = m.WSSPort.ValueInt64()
	}
	if !m.TLSOnly.IsNull() && !m.TLSOnly.IsUnknown() {
		b["tls_only"] = m.TLSOnly.ValueBool()
	}
	return b
}

func (m *serverSettingsModel) set(s client.ServerSettings) {
	m.ID = types.StringValue("settings")
	m.ODBCDSN = types.StringValue(s.ODBCDSN)
	m.DBUser = types.StringValue(s.DBUser)
	m.AMIUser = types.StringValue(s.AMIUser)
	m.AMIBind = types.StringValue(s.AMIBind)
	m.AMIPort = types.Int64Value(int64(s.AMIPort))
	m.SIPPort = types.Int64Value(int64(s.SIPPort))
	m.IAXPort = types.Int64Value(int64(s.IAXPort))
	m.ExternalIP = types.StringValue(s.ExternalIP)
	m.LocalNet = types.StringValue(s.LocalNet)
	m.RTPStart = types.Int64Value(int64(s.RTPStart))
	m.RTPEnd = types.Int64Value(int64(s.RTPEnd))
	m.VoicemailFrom = types.StringValue(s.VoicemailFrom)
	m.MailCmd = types.StringValue(s.MailCmd)
	m.MediaURL = types.StringValue(s.MediaURL)
	m.RecordingsDir = types.StringValue(s.RecordingsDir)
	m.TLSCertFile = types.StringValue(s.TLSCertFile)
	m.TLSKeyFile = types.StringValue(s.TLSKeyFile)
	m.TLSPort = types.Int64Value(int64(s.TLSPort))
	m.WSSPort = types.Int64Value(int64(s.WSSPort))
	m.TLSOnly = types.BoolValue(s.TLSOnly)
}

func (r *serverSettingsResource) write(ctx context.Context, m *serverSettingsModel) error {
	st, err := r.c.UpdateServerSettings(ctx, m.body())
	if err != nil {
		return err
	}
	m.set(st.Settings)
	return nil
}

func (r *serverSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m serverSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &m); err != nil {
		resp.Diagnostics.AddError("set the server settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *serverSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m serverSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &m); err != nil {
		resp.Diagnostics.AddError("update the server settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *serverSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m serverSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	st, err := r.c.GetServerSettings(ctx)
	if err != nil {
		readFailed(ctx, "server settings", err, resp)
		return
	}
	m.set(st.Settings) // the secret is not returned: the state keeps what the configuration last set
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *serverSettingsResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	// the settings belong to the server: destroying the resource only stops managing them
}

func (r *serverSettingsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "settings")...)
}
