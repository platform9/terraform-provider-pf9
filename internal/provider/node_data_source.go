package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_node"
)

var _ datasource.DataSource = (*nodeDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*nodeDataSource)(nil)

func NewNodeDataSource() datasource.DataSource {
	return &nodeDataSource{}
}

type nodeDataSource struct {
	client *pmk.HTTPClient
}

func (d *nodeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

func (d *nodeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_node.NodeDataSourceSchema(ctx)
}

func (d *nodeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *nodeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_node.NodeModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	nodeID := data.Id.ValueString()

	// Read API call logic
	authInfo, err := d.client.Authenticator().Auth(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID

	nodes, err := d.client.Qbert().ListNodes(projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list nodes", err.Error())
		return
	}
	for _, node := range nodes {
		if node.UUID == nodeID {
			data.Id = types.StringValue(node.UUID)
			data.Name = types.StringValue(node.Name)
			data.Status = types.StringValue(node.Status)
			data.ClusterName = types.StringValue(node.ClusterName)
			data.ClusterUuid = types.StringValue(node.ClusterUUID)
			data.NodePoolName = types.StringValue(node.NodePoolName)
			data.NodePoolUuid = types.StringValue(node.NodePoolUUID)
			data.IsMaster = types.BoolValue(node.IsMaster != 0)
			data.ApiResponding = types.BoolValue(node.APIResponding != 0)
			data.ProjectId = types.StringValue(node.ProjectID)
			data.ActualKubeRoleVersion = types.StringValue(node.ActualKubeRoleVersion)
			break
		}
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
