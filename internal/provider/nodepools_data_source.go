package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/platform9/pf9-sdk-go/pf9/keystone"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_nodepools"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_nodes"
	// "github.com/platform9/terraform-provider-pf9/internal/provider/datasource_nodes"
)

var _ datasource.DataSource = (*nodepoolsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*nodepoolsDataSource)(nil)

func NewNodepoolsDataSource() datasource.DataSource {
	return &nodepoolsDataSource{}
}

type nodepoolsDataSource struct {
	client *pmk.HTTPClient
}

func (d *nodepoolsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nodepools"
}

func (d *nodepoolsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_nodepools.NodepoolsDataSourceSchema(ctx)
}

func (d *nodepoolsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *nodepoolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data, state datasource_nodepools.NodepoolsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	var values []string
	name := data.Filter.Name.ValueString()
	resp.Diagnostics.Append(data.Filter.Values.ElementsAs(ctx, &values, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	authInfo, err := d.client.Authenticator().Auth(ctx, keystone.AuthOptions{
		PropagateCacheErrors: true,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID
	nodePools, err := d.client.Qbert().GetNodePools(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get nodepools", err.Error())
		return
	}
	var filteredNodePools []qbert.NodePool
	for _, nodepool := range nodePools {
		if name == "name" {
			if StrSliceContains(values, nodepool.Name) {
				filteredNodePools = append(filteredNodePools, nodepool)
			}
		} else if name == "cloudProviderName" {
			if StrSliceContains(values, nodepool.CloudProviderName) {
				filteredNodePools = append(filteredNodePools, nodepool)
			}
		} else if name == "cloudProviderUuid" {
			if StrSliceContains(values, nodepool.CloudProviderUUID) {
				filteredNodePools = append(filteredNodePools, nodepool)
			}
		} else {
			resp.Diagnostics.AddError("Invalid filter name",
				"Allowed filter names are: name, id, primary_ip, is_master, api_responding, cluster_name, cluster_uuid, node_pool_name, node_pool_uuid")
			return
		}
	}
	var tfNodePools []attr.Value
	for _, nodepool := range filteredNodePools {
		tfNodePoolsVal := datasource_nodepools.NodepoolsValue{
			Id:                types.StringValue(nodepool.UUID),
			Name:              types.StringValue(nodepool.Name),
			CloudProviderName: types.StringValue(nodepool.CloudProviderName),
			CloudProviderUuid: types.StringValue(nodepool.CloudProviderUUID),
		}
		tfNodesObjVal, diags := tfNodePoolsVal.ToObjectValue(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		tfNodesAttrVal, diags := datasource_nodepools.NewNodepoolsValue(tfNodesObjVal.AttributeTypes(ctx), tfNodesObjVal.Attributes())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		tfNodePools = append(tfNodePools, tfNodesAttrVal)
	}

	// data value setting
	state.Id = types.StringValue(data.Id.ValueString())
	nodePoolsListVal, diags := types.ListValue(datasource_nodes.NodesValue{}.Type(ctx), tfNodePools)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Nodepools = nodePoolsListVal

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
