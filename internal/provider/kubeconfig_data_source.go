package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_kubeconfig"
)

var _ datasource.DataSource = (*kubeconfigDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*kubeconfigDataSource)(nil)

func NewKubeconfigDataSource() datasource.DataSource {
	return &kubeconfigDataSource{}
}

type kubeconfigDataSource struct {
	client *pmk.HTTPClient
}

func (d *kubeconfigDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

func (d *kubeconfigDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_kubeconfig.KubeconfigDataSourceSchema(ctx)
}

func (d *kubeconfigDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *kubeconfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_kubeconfig.KubeconfigModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	authInfo, err := d.client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}

	clusterID := data.Id.ValueString()
	forceCertAuth := data.ForceCertAuth.ValueBool()
	projectID := authInfo.ProjectID
	kubeconfigBlob, err := d.client.Qbert().GetClusterKubeconfig(projectID, clusterID, authInfo.Token, qbert.KubeconfigOptions{ForceCertAuth: forceCertAuth})
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster kubeconfig", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster kubeconfig", err.Error())
		return
	}
	kubeconfigStr := string(kubeconfigBlob)
	data.Raw = types.StringValue(kubeconfigStr)

	// TODO: Set the following fields correctly
	data.Endpoint = types.StringValue("https://api." + clusterID + ".pf9.io")
	data.ClusterCaCertificate = types.StringValue("")
	data.Token = types.StringValue(authInfo.Token)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
