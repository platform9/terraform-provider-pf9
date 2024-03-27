package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
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

	// TODO: Read API call logic

	// Example data value setting
	data.Id = types.StringValue("example-id")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
