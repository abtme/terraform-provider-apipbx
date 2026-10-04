package provider

import (
	"context"
	"net/netip"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

// ---- shared: sources

// normSource is how apipbxd writes an address: a CIDR range, a lone address becoming /32 or /128. A configuration may say
// "198.51.100.7" and the server answer "198.51.100.7/32": they are the same, and must not look like a change.
func normSource(s string) string {
	if p, err := netip.ParsePrefix(s); err == nil {
		return p.Masked().String()
	}
	if a, err := netip.ParseAddr(s); err == nil {
		a = a.Unmap()
		return netip.PrefixFrom(a, a.BitLen()).String()
	}
	return s
}

// sourceSet makes the state's set from what the server answered, keeping the way the user wrote each address when it means the same.
func sourceSet(fromServer []string, planned types.Set) types.Set {
	user := map[string]string{}
	for _, e := range planned.Elements() {
		if s, ok := e.(types.String); ok {
			user[normSource(s.ValueString())] = s.ValueString()
		}
	}
	vals := make([]attr.Value, 0, len(fromServer))
	for _, s := range fromServer {
		if u, ok := user[normSource(s)]; ok {
			s = u
		}
		vals = append(vals, types.StringValue(s))
	}
	return types.SetValueMust(types.StringType, vals)
}

func stringsOf(s types.Set) []string {
	var out []string
	for _, e := range s.Elements() {
		out = append(out, e.(types.String).ValueString())
	}
	return out
}

var emptySet = types.SetValueMust(types.StringType, []attr.Value{})

// sameSourceSets says whether two sets name the same addresses once each is written the way the server writes it.
func sameSourceSets(a, b types.Set) bool {
	na := map[string]bool{}
	for _, e := range a.Elements() {
		na[normSource(e.(types.String).ValueString())] = true
	}
	nb := map[string]bool{}
	for _, e := range b.Elements() {
		nb[normSource(e.(types.String).ValueString())] = true
	}
	if len(na) != len(nb) {
		return false
	}
	for k := range na {
		if !nb[k] {
			return false
		}
	}
	return true
}

// sameSources keeps the stored value when the configuration means the same addresses, so "198.51.100.7" against a stored
// "198.51.100.7/32" is no change: after an import (which has no configuration to copy the spelling from) it would otherwise force
// a rule to be replaced for nothing.
type sameSources struct{}

func (sameSources) Description(context.Context) string {
	return "Addresses written differently but meaning the same are not a change."
}
func (m sameSources) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }
func (sameSources) PlanModifySet(_ context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if sameSourceSets(req.StateValue, req.PlanValue) {
		resp.PlanValue = req.StateValue
	}
}

// ---- apipbx_firewall

type firewallResource struct{ c *client.Client }

type firewallModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func NewFirewallResource() resource.Resource { return &firewallResource{} }

func (r *firewallResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall"
}

func (r *firewallResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Turns the server's firewall on or off (needs a platform key). Off, the network is left exactly as it is; on, a default-drop ruleset is enforced: " +
			"the services, restrictions, rules and carriers (see the other apipbx_firewall_* resources) decide what gets through. Turning it on is refused when a " +
			"restriction of SSH or the API would lock the caller out. Destroying this resource turns the firewall off. Import with \"firewall\".",
		Attributes: map[string]schema.Attribute{
			"id":      idAttr(),
			"enabled": schema.BoolAttribute{Required: true, Description: "Enforce the firewall."},
		}}
}

func (r *firewallResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (r *firewallResource) apply(ctx context.Context, m *firewallModel, diags interface {
	AddError(string, string)
}) {
	var err error
	if m.Enabled.ValueBool() {
		_, err = r.c.EnableFirewall(ctx)
	} else {
		_, err = r.c.DisableFirewall(ctx)
	}
	if err != nil {
		diags.AddError("set the firewall", err.Error())
		return
	}
	m.ID = types.StringValue("firewall")
}

func (r *firewallResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m firewallModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &m, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *firewallResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m firewallModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &m, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *firewallResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m firewallModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	f, err := r.c.GetFirewall(ctx)
	if err != nil {
		readFailed(ctx, "firewall", err, resp)
		return
	}
	m.ID, m.Enabled = types.StringValue("firewall"), types.BoolValue(f.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if _, err := r.c.DisableFirewall(ctx); err != nil {
		deleteFailed("firewall", err, resp)
	}
}

func (r *firewallResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "firewall")...)
}

// ---- apipbx_firewall_restriction

type firewallRestrictionResource struct{ c *client.Client }

type firewallRestrictionModel struct {
	ID      types.String `tfsdk:"id"`
	Service types.String `tfsdk:"service"`
	Sources types.Set    `tfsdk:"sources"`
	Comment types.String `tfsdk:"comment"`
	Force   types.Bool   `tfsdk:"force"`
}

func NewFirewallRestrictionResource() resource.Resource { return &firewallRestrictionResource{} }

func (r *firewallRestrictionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_restriction"
}

func (r *firewallRestrictionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Limits a service of the server's firewall to chosen sources: only they reach it, everyone else is dropped (while the firewall is on). " +
			"The voice services (sip, sips, wss, iax2, rtp) stay open to enabled carriers and to your trunks' hosts. A family with no listed source is closed, " +
			"so restricting to an IPv4 address also closes the service over IPv6. For ssh and api the sources must include the address Terraform runs from " +
			"(the connection's own address: behind a proxy, the proxy's), or the change is refused. Import with the service id.",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(),
			"service": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "ssh, api, https, sip, sips, wss, iax2 or rtp."},
			"sources": schema.SetAttribute{Required: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Set{sameSources{}},
				Description: "Addresses or CIDR ranges, IPv4 and IPv6 mixed. Never 0.0.0.0/0 or ::/0 (delete the restriction instead)."},
			"comment": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"force":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "ssh only: accept sources that do not include the caller (the SSH client and Terraform are not always on the same machine)."},
		}}
}

func (r *firewallRestrictionResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *firewallRestrictionModel) set(f client.FirewallRestriction) {
	m.ID, m.Service = types.StringValue(f.Service), types.StringValue(f.Service)
	m.Sources = sourceSet(f.Sources, m.Sources)
	m.Comment, m.Force = types.StringValue(f.Comment), types.BoolValue(f.Force)
}

func (r *firewallRestrictionResource) put(ctx context.Context, m *firewallRestrictionModel) error {
	f, err := r.c.SetFirewallRestriction(ctx, m.Service.ValueString(), client.FirewallRestrictionInput{Sources: stringsOf(m.Sources), Comment: m.Comment.ValueString(), Force: m.Force.ValueBool()})
	if err != nil {
		return err
	}
	m.set(f)
	return nil
}

func (r *firewallRestrictionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m firewallRestrictionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("create firewall restriction", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallRestrictionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m firewallRestrictionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("update firewall restriction", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallRestrictionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m firewallRestrictionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	f, err := r.c.GetFirewallRestriction(ctx, m.ID.ValueString())
	if err != nil {
		readFailed(ctx, "firewall restriction", err, resp)
		return
	}
	m.set(f)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallRestrictionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m firewallRestrictionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	deleteFailed("firewall restriction", r.c.DeleteFirewallRestriction(ctx, m.ID.ValueString()), resp)
}

func (r *firewallRestrictionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service"), req.ID)...)
}

// ---- apipbx_firewall_rule

type firewallRuleResource struct{ c *client.Client }

type firewallRuleModel struct {
	ID       types.String `tfsdk:"id"`
	Action   types.String `tfsdk:"action"`
	Protocol types.String `tfsdk:"protocol"`
	Port     types.String `tfsdk:"port"`
	Sources  types.Set    `tfsdk:"sources"`
	Comment  types.String `tfsdk:"comment"`
}

func NewFirewallRuleResource() resource.Resource { return &firewallRuleResource{} }

func (r *firewallRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_rule"
}

func (r *firewallRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "An extra accept, drop or reject rule, which runs before the services. A drop or reject that would block the caller from SSH or the API is refused: " +
			"to let only some addresses in, use apipbx_firewall_restriction. apipbxd has no way to change a rule, so every attribute forces a replacement. Import with the rule id.",
		Attributes: map[string]schema.Attribute{
			"id":       idAttr(),
			"action":   schema.StringAttribute{Required: true, PlanModifiers: replaceStr, Description: "accept, drop or reject."},
			"protocol": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("any"), PlanModifiers: replaceStr, Description: "tcp, udp or any (a port needs tcp or udp)."},
			"port":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: replaceStr, Description: "A port such as 5060 or a range such as 10000-20000; empty is every port."},
			"sources": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(emptySet),
				PlanModifiers: []planmodifier.Set{sameSources{}, setplanmodifier.RequiresReplace()}, Description: "Addresses or CIDR ranges; empty is from anywhere."},
			"comment": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: replaceStr},
		}}
}

func (r *firewallRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *firewallRuleModel) set(f client.FirewallRule) {
	m.ID = idString(f.ID)
	m.Action, m.Protocol, m.Port, m.Comment = types.StringValue(f.Action), types.StringValue(f.Protocol), types.StringValue(f.Port), types.StringValue(f.Comment)
	m.Sources = sourceSet(f.Sources, m.Sources)
}

func (r *firewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m firewallRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	f, err := r.c.CreateFirewallRule(ctx, client.FirewallRuleInput{Action: m.Action.ValueString(), Protocol: m.Protocol.ValueString(), Port: m.Port.ValueString(), Sources: stringsOf(m.Sources), Comment: m.Comment.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create firewall rule", err.Error())
		return
	}
	m.set(f)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallRuleResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	// every attribute forces a replacement, so there is nothing to update
}

func (r *firewallRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m firewallRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id, err := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("read firewall rule", "the id is not a number")
		return
	}
	f, err := r.c.GetFirewallRule(ctx, id)
	if err != nil {
		readFailed(ctx, "firewall rule", err, resp)
		return
	}
	m.set(f)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m firewallRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("firewall rule", r.c.DeleteFirewallRule(ctx, id), resp)
}

func (r *firewallRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// ---- apipbx_firewall_carrier

type firewallCarrierResource struct{ c *client.Client }

type firewallCarrierModel struct {
	ID       types.String `tfsdk:"id"`
	Provider types.String `tfsdk:"provider_id"`
	Name     types.String `tfsdk:"name"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Sources  types.Set    `tfsdk:"sources"`
	Hosts    types.Set    `tfsdk:"hosts"`
}

func NewFirewallCarrierResource() resource.Resource { return &firewallCarrierResource{} }

func (r *firewallCarrierResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_carrier"
}

func (r *firewallCarrierResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A carrier whose servers always reach the voice services (SIP, SIP over TLS, WebRTC, IAX2 and RTP), even when those are restricted to your own networks. " +
			"Use a built-in template (provider_id = \"magrathea\") or a custom carrier (provider_id = \"custom\" with sources and/or hosts). " +
			"A template brings the provider's gateways and every address block it announces; host names are resolved every few minutes. Import with the carrier id.",
		Attributes: map[string]schema.Attribute{
			"id": idAttr(),
			"provider_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "A template id (magrathea) or custom. GET /v1/firewall/providers lists the templates."},
			"name": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description: "Defaults to the template's name."},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"sources": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(emptySet), PlanModifiers: []planmodifier.Set{sameSources{}},
				Description: "Added to the template's address blocks; all there is for a custom carrier."},
			"hosts": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(emptySet),
				Description: "Host names (resolved) or addresses, added to the template's gateways."},
		}}
}

func (r *firewallCarrierResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *firewallCarrierModel) set(f client.FirewallCarrier) {
	m.ID, m.Provider, m.Name, m.Enabled = idString(f.ID), types.StringValue(f.Provider), types.StringValue(f.Name), types.BoolValue(f.Enabled)
	m.Sources = sourceSet(f.Sources, m.Sources)
	hosts := make([]attr.Value, 0, len(f.Hosts))
	for _, h := range f.Hosts {
		hosts = append(hosts, types.StringValue(h))
	}
	m.Hosts = types.SetValueMust(types.StringType, hosts)
}

func (r *firewallCarrierResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m firewallCarrierModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.FirewallCarrierInput{Provider: m.Provider.ValueString(), Enabled: ptr(m.Enabled.ValueBool()), Sources: stringsOf(m.Sources), Hosts: stringsOf(m.Hosts)}
	if !m.Name.IsUnknown() && !m.Name.IsNull() {
		in.Name = m.Name.ValueString()
	}
	f, err := r.c.CreateFirewallCarrier(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("create firewall carrier", err.Error())
		return
	}
	m.set(f)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallCarrierResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m firewallCarrierModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	f, err := r.c.GetFirewallCarrier(ctx, id)
	if err != nil {
		readFailed(ctx, "firewall carrier", err, resp)
		return
	}
	m.set(f)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallCarrierResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m firewallCarrierModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	src, hosts := stringsOf(m.Sources), stringsOf(m.Hosts)
	if src == nil {
		src = []string{}
	}
	if hosts == nil {
		hosts = []string{}
	}
	f, err := r.c.UpdateFirewallCarrier(ctx, id, client.FirewallCarrierPatch{Name: ptr(m.Name.ValueString()), Enabled: ptr(m.Enabled.ValueBool()), Sources: &src, Hosts: &hosts})
	if err != nil {
		resp.Diagnostics.AddError("update firewall carrier", err.Error())
		return
	}
	m.set(f)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallCarrierResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m firewallCarrierModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("firewall carrier", r.c.DeleteFirewallCarrier(ctx, id), resp)
}

func (r *firewallCarrierResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// ---- apipbx_firewall_site

type firewallSiteResource struct{ c *client.Client }

type firewallSiteModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Host      types.String `tfsdk:"host"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	Services  types.Set    `tfsdk:"services"`
	Addresses types.Set    `tfsdk:"addresses"`
}

func NewFirewallSiteResource() resource.Resource { return &firewallSiteResource{} }

func (r *firewallSiteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_site"
}

func (r *firewallSiteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A connection on a dynamic address (an office or home whose ISP renumbers it), kept as a DNS name that follows it. Its current addresses are let " +
			"through every restricted service of the firewall (or only the listed ones), and the ruleset is rewritten when the name's address changes " +
			"(checked every minute; a lookup that fails keeps the last address). Import with the site id.",
		Attributes: map[string]schema.Attribute{
			"id":      idAttr(),
			"name":    schema.StringAttribute{Required: true},
			"host":    schema.StringAttribute{Required: true, Description: "The DNS name (dynamic DNS, or any name kept pointing at the connection)."},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"services": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(emptySet),
				Description: "ssh, api, https, sip, sips, wss, iax2 or rtp; empty is every restricted service."},
			"addresses": schema.SetAttribute{Computed: true, ElementType: types.StringType, Description: "What the name resolves to now."},
		}}
}

func (r *firewallSiteResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *firewallSiteModel) set(s client.FirewallSite) {
	m.ID, m.Name, m.Host, m.Enabled = idString(s.ID), types.StringValue(s.Name), types.StringValue(s.Host), types.BoolValue(s.Enabled)
	sv := make([]attr.Value, 0, len(s.Services))
	for _, x := range s.Services {
		sv = append(sv, types.StringValue(x))
	}
	m.Services = types.SetValueMust(types.StringType, sv)
	av := make([]attr.Value, 0, len(s.Addresses))
	for _, x := range s.Addresses {
		av = append(av, types.StringValue(x))
	}
	m.Addresses = types.SetValueMust(types.StringType, av)
}

func (r *firewallSiteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m firewallSiteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.c.CreateFirewallSite(ctx, client.FirewallSiteInput{Name: m.Name.ValueString(), Host: m.Host.ValueString(), Enabled: ptr(m.Enabled.ValueBool()), Services: stringsOf(m.Services)})
	if err != nil {
		resp.Diagnostics.AddError("create firewall site", err.Error())
		return
	}
	// the addresses were resolved by the create; read them back
	if got, err := r.c.GetFirewallSite(ctx, s.ID); err == nil {
		s = got
	}
	m.set(s)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallSiteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m firewallSiteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.c.GetFirewallSite(ctx, id)
	if err != nil {
		readFailed(ctx, "firewall site", err, resp)
		return
	}
	m.set(s)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallSiteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m firewallSiteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	svc := stringsOf(m.Services)
	if svc == nil {
		svc = []string{}
	}
	if _, err := r.c.UpdateFirewallSite(ctx, id, client.FirewallSitePatch{Name: ptr(m.Name.ValueString()), Host: ptr(m.Host.ValueString()), Enabled: ptr(m.Enabled.ValueBool()), Services: &svc}); err != nil {
		resp.Diagnostics.AddError("update firewall site", err.Error())
		return
	}
	s, err := r.c.GetFirewallSite(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("read firewall site", err.Error())
		return
	}
	m.set(s)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallSiteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m firewallSiteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	id := parseID(&resp.Diagnostics, "id", m.ID)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteFailed("firewall site", r.c.DeleteFirewallSite(ctx, id), resp)
}

func (r *firewallSiteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
