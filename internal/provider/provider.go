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
	tflog.Debug(ctx, "Configuring client")

	var accountURL, username, password, region, tenant string
	var pf9Model provider_pf9.Pf9Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &pf9Model)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to get config", map[string]interface{}{"diagnostics": resp.Diagnostics})
		return
	}

	if !pf9Model.AccountUrl.IsNull() {
		accountURL = pf9Model.AccountUrl.ValueString()
	} else {
		resp.Diagnostics.AddAttributeError(path.Root("account_url"), "Account URL is required", "Account URL is required")
	}

	if !pf9Model.Username.IsNull() {
		username = pf9Model.Username.ValueString()
	} else {
		resp.Diagnostics.AddAttributeError(path.Root("username"), "Username is required", "Username is required")
	}
	if !pf9Model.Password.IsNull() {
		password = pf9Model.Password.ValueString()
	} else {
		resp.Diagnostics.AddAttributeError(path.Root("password"), "Password is required", "Password is required")
	}

	if pf9Model.Region.IsNull() {
		region = "RegionOne"
	} else {
		region = pf9Model.Region.ValueString()
	}

	if pf9Model.Tenant.IsNull() {
		tenant = "service"
	} else {
		tenant = pf9Model.Tenant.ValueString()
	}
	if resp.Diagnostics.HasError() {
		return
	}

	unAuthenticatedClient := pmk.NewClient(accountURL)
	if err := unAuthenticatedClient.Ping(ctx); err != nil {
		tflog.Error(ctx, "Failed to ping")
		resp.Diagnostics.AddError("Failed to ping", err.Error())
		return
	}
	// Setting this env variable to be used in kubeconfig data source
	err := os.Setenv("DU_USERNAME", username)
	if err != nil {
		resp.Diagnostics.AddError("Failed to set env variable", "env variable DU_USERNAME cant be set")
		return
	}
	// Setting this env variable to be used in kubeconfig data source
	err = os.Setenv("DU_PASSWORD", password)
	if err != nil {
		resp.Diagnostics.AddError("Failed to set env variable", "env variable DU_PASSWORD cant be set")
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
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	tflog.Debug(ctx, "Client authenticated AuthInfo: %v", map[string]interface{}{"authInfo": authInfo})
	resp.ResourceData = client
	resp.DataSourceData = client
	tflog.Info(ctx, "Client configured", map[string]interface{}{"accountURL": accountURL, "auth.userID": authInfo.UserID,
		"auth.projectID": authInfo.ProjectID, "username": username, "tenant": tenant, "region": region})
}

func (p *pf9Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pf9"
	resp.Version = p.Version
}

func (p *pf9Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewNodeDataSource,
		NewNodesDataSource,
		NewNodepoolsDataSource,
		NewClusterDataSource,
		NewClustersDataSource,
		NewKubeconfigDataSource,
		NewHostDataSource,
		NewHostsDataSource,
	}
}

func (p *pf9Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewClusterResource,
	}
}
