package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/platform9/pf9-sdk-go/pf9/keystone"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/terraform-provider-pf9/internal/provider/provider_pf9"
)

var _ provider.Provider = (*pf9Provider)(nil)

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &pf9Provider{
			Version: version,
		}
	}
}

type pf9Provider struct {
	Version string
}

func (p *pf9Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = provider_pf9.Pf9ProviderSchema(ctx)
}

func (p *pf9Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring client")

	duFQDN := os.Getenv("DU_FQDN")
	username := os.Getenv("DU_USERNAME")
	password := os.Getenv("DU_PASSWORD")
	tenant := os.Getenv("DU_TENANT")
	region := os.Getenv("DU_REGION")

	var pf9Model provider_pf9.Pf9Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &pf9Model)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to get config", map[string]interface{}{"diagnostics": resp.Diagnostics})
		return
	}

	if duFQDN == "" {
		if pf9Model.DuFqdn.IsNull() {
			tflog.Error(ctx, "DU_FQDN is empty")
			resp.Diagnostics.AddAttributeError(path.Root("du_fqdn"), "DU_FQDN is empty", "DU_FQDN is empty")
		}
		duFQDN = pf9Model.DuFqdn.ValueString()
	}
	if username == "" {
		if pf9Model.DuUsername.IsNull() {
			tflog.Error(ctx, "DU_USERNAME is empty")
			resp.Diagnostics.AddAttributeError(path.Root("du_username"), "DU_USERNAME is empty", "DU_USERNAME is empty")
		}
		username = pf9Model.DuUsername.ValueString()
	}
	if region == "" {
		if pf9Model.DuRegion.IsNull() {
			region = "RegionOne"
		} else {
			region = pf9Model.DuRegion.ValueString()
		}
	}
	if tenant == "" {
		if pf9Model.DuTenant.IsNull() {
			tenant = "service"
		} else {
			tenant = pf9Model.DuTenant.String()
		}
	}
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "One or more required attributes are empty")
		return
	}

	unAuthenticatedClient := pmk.NewClient(duFQDN)
	if err := unAuthenticatedClient.Ping(ctx); err != nil {
		tflog.Error(ctx, "Failed to ping")
		resp.Diagnostics.AddError("Failed to ping", err.Error())
		return
	}
	client := unAuthenticatedClient.WithCredentials(keystone.Credentials{
		Username: username,
		Password: password,
		Tenant:   tenant,
		Region:   region,
	})
	authInfo, err := client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	tflog.Info(ctx, "Client authenticated AuthInfo: %v", map[string]interface{}{"authInfo": authInfo})
	resp.ResourceData = client
	resp.DataSourceData = client
	tflog.Info(ctx, "Client configured", map[string]interface{}{"client": client})
}

func (p *pf9Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pf9"
	resp.Version = p.Version
}

func (p *pf9Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewKubeconfigDataSource,
		NewNodeDataSource,
		NewNodesDataSource,
		NewNodepoolsDataSource,
		NewClustersDataSource,
	}
}

func (p *pf9Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewClusterResource,
	}
}
