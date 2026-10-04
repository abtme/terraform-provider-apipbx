package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

// apipbx_firewall_provider: a carrier template of your own, used by apipbx_firewall_carrier.provider_id like a built-in one.

type firewallProviderResource struct{ c *client.Client }

type firewallProviderModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Hosts       types.List   `tfsdk:"hosts"`
	Prefixes    types.Set    `tfsdk:"prefixes"`
	Notes       types.List   `tfsdk:"notes"`
	Verified    types.String `tfsdk:"verified"`
}

var providerHostType = types.ObjectType{AttrTypes: map[string]attr.Type{"host": types.StringType, "use": types.StringType}}

func NewFirewallProviderResource() resource.Resource { return &firewallProviderResource{} }

func (r *firewallProviderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_provider"
}

func (r *firewallProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A carrier template of your own: a provider's gateways and address blocks, kept once and used by carriers (apipbx_firewall_carrier, " +
			"provider_id = this id) like the built-in ones such as magrathea. Changing it changes every carrier that uses it. The built-in templates cannot be " +
			"managed here. A template that a carrier still uses cannot be destroyed. Import with the id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "a-z 0-9 -, starts with a letter, at most 32; not custom or a built-in id."},
			"name":        schema.StringAttribute{Required: true},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"hosts": schema.ListNestedAttribute{Optional: true, Computed: true, Description: "The provider's gateways; their addresses are looked up every minute.",
				Default: listdefault.StaticValue(types.ListValueMust(providerHostType, []attr.Value{})),
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{Required: true},
					"use":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "What the gateway is for (SIP, IAX2...)."},
				}}},
			"prefixes": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(emptySet),
				PlanModifiers: []planmodifier.Set{sameSources{}}, Description: "Every address block the provider announces (IPv4 and IPv6). At least one host or block is needed."},
			"notes": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
				Description: "Where the data comes from."},
			"verified": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "When it was last checked: addresses go stale."},
		}}
}

func (r *firewallProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	clientFrom(req.ProviderData, &r.c)
}

func (m *firewallProviderModel) hosts() []client.FirewallProviderHost {
	out := []client.FirewallProviderHost{}
	for _, e := range m.Hosts.Elements() {
		o := e.(types.Object).Attributes()
		out = append(out, client.FirewallProviderHost{Host: o["host"].(types.String).ValueString(), Use: o["use"].(types.String).ValueString()})
	}
	return out
}

func (m *firewallProviderModel) notes() []string {
	out := []string{}
	for _, e := range m.Notes.Elements() {
		out = append(out, e.(types.String).ValueString())
	}
	return out
}

func (m *firewallProviderModel) set(p client.FirewallProvider) {
	m.ID, m.Name, m.Description, m.Verified = types.StringValue(p.ID), types.StringValue(p.Name), types.StringValue(p.Description), types.StringValue(p.Verified)
	hv := make([]attr.Value, 0, len(p.Hosts))
	for _, h := range p.Hosts {
		hv = append(hv, types.ObjectValueMust(providerHostType.AttrTypes, map[string]attr.Value{"host": types.StringValue(h.Host), "use": types.StringValue(h.Use)}))
	}
	m.Hosts = types.ListValueMust(providerHostType, hv)
	m.Prefixes = sourceSet(p.Prefixes, m.Prefixes)
	nv := make([]attr.Value, 0, len(p.Notes))
	for _, n := range p.Notes {
		nv = append(nv, types.StringValue(n))
	}
	m.Notes = types.ListValueMust(types.StringType, nv)
}

func (r *firewallProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m firewallProviderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.CreateFirewallProvider(ctx, client.FirewallProviderInput{ID: m.ID.ValueString(), Name: m.Name.ValueString(), Description: m.Description.ValueString(),
		Hosts: m.hosts(), Prefixes: stringsOf(m.Prefixes), Notes: m.notes(), Verified: m.Verified.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("create firewall provider", err.Error())
		return
	}
	m.set(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m firewallProviderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	p, err := r.c.GetFirewallProvider(ctx, m.ID.ValueString())
	if err != nil {
		readFailed(ctx, "firewall provider", err, resp)
		return
	}
	m.set(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m firewallProviderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hosts, pre, notes := m.hosts(), stringsOf(m.Prefixes), m.notes()
	if pre == nil {
		pre = []string{}
	}
	p, err := r.c.UpdateFirewallProvider(ctx, m.ID.ValueString(), client.FirewallProviderPatch{Name: ptr(m.Name.ValueString()), Description: ptr(m.Description.ValueString()),
		Hosts: &hosts, Prefixes: &pre, Notes: &notes, Verified: ptr(m.Verified.ValueString())})
	if err != nil {
		resp.Diagnostics.AddError("update firewall provider", err.Error())
		return
	}
	m.set(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *firewallProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m firewallProviderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	deleteFailed("firewall provider", r.c.DeleteFirewallProvider(ctx, m.ID.ValueString()), resp)
}

func (r *firewallProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
