// Package provider implements the apipbx Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

type apipbxProvider struct{ version string }

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &apipbxProvider{version: version} }
}

func (p *apipbxProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "apipbx"
	resp.Version = p.version
}

func (p *apipbxProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"endpoint": schema.StringAttribute{Optional: true, Description: "apipbxd base URL (or APIPBX_ENDPOINT)."},
		"token":    schema.StringAttribute{Optional: true, Sensitive: true, Description: "API key (or APIPBX_TOKEN)."},
	}}
}

func (p *apipbxProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg struct {
		Endpoint types.String `tfsdk:"endpoint"`
		Token    types.String `tfsdk:"token"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	endpoint, token := cfg.Endpoint.ValueString(), cfg.Token.ValueString()
	if endpoint == "" {
		endpoint = os.Getenv("APIPBX_ENDPOINT")
	}
	if token == "" {
		token = os.Getenv("APIPBX_TOKEN")
	}
	if endpoint == "" || token == "" {
		resp.Diagnostics.AddError("missing configuration", "endpoint and token (or APIPBX_ENDPOINT / APIPBX_TOKEN) are required")
		return
	}
	c := client.New(endpoint, token)
	resp.ResourceData, resp.DataSourceData = c, c
}

func (p *apipbxProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTenantResource, NewAPIKeyResource, NewExtensionResource, NewTrunkResource,
		NewRingGroupResource, NewInboundRouteResource, NewOutboundRouteResource, NewVoicemailBoxResource,
		NewTimeGroupResource, NewTimeConditionResource, NewIVRResource, NewRecordingResource, NewVoicemailGreetingResource,
		NewBlacklistEntryResource, NewBlacklistSettingsResource, NewQueueResource, NewConferenceResource,
	}
}

func (p *apipbxProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }
