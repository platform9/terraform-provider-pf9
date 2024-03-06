package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/platform9/pf9-sdk-go/pf9/keystone"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_cluster"
)

var _ datasource.DataSource = (*clusterDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*clusterDataSource)(nil)

func NewClusterDataSource() datasource.DataSource {
	return &clusterDataSource{}
}

type clusterDataSource struct {
	client *pmk.HTTPClient
}

func (d *clusterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (d *clusterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_cluster.ClusterDataSourceSchema(ctx)
}

func (d *clusterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *clusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_cluster.ClusterModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	authInfo, err := d.client.Authenticator().Auth(ctx, keystone.AuthOptions{
		PropagateCacheErrors: true,
	})
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}

	clusterID := data.Id.ValueString()
	projectID := authInfo.ProjectID
	cluster, err := d.client.Qbert().GetCluster(ctx, projectID, clusterID)
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster", err.Error())
		return
	}
	kubeconfigBlob, err := d.client.Qbert().GetClusterKubeconfig(projectID, clusterID, authInfo.Token)
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster kubeconfig", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster kubeconfig", err.Error())
		return
	}
	kubeconfigStr := string(kubeconfigBlob)

	diags := d.ToTerraformType(ctx, cluster, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	data.Kubeconfig = types.StringValue(kubeconfigStr)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *clusterDataSource) ToTerraformType(ctx context.Context, in *qbert.Cluster, out *datasource_cluster.ClusterModel) diag.Diagnostics {

	out.Id = types.StringValue(in.UUID)
	out.Name = types.StringValue(in.Name)
	out.ProjectId = types.StringValue(in.ProjectId)
	out.WorkerStatus = types.StringValue(in.WorkerStatus)
	out.MasterStatus = types.StringValue(in.MasterStatus)
	out.Status = types.StringValue(in.Status)
	out.TaskError = types.StringValue(in.TaskError)
	out.TaskStatus = types.StringValue(in.TaskStatus)

	return diag.Diagnostics{}
}
