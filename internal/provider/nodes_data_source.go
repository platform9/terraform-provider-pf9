package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/platform9/pf9-sdk-go/pf9/keystone"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_nodes"
)

var _ datasource.DataSource = (*nodesDataSource)(nil)

func NewNodesDataSource() datasource.DataSource {
	return &nodesDataSource{}
}

type nodesDataSource struct {
	client *pmk.HTTPClient
}

func (d *nodesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nodes"
}

func (d *nodesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_nodes.NodesDataSourceSchema(ctx)
}

func (d *nodesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *nodesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data, state datasource_nodes.NodesModel

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
	nodesList, err := d.client.Qbert().ListNodes(projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list nodes", err.Error())
		return
	}
	var nodes []qbert.Node
	for _, node := range nodesList {
		if name == "name" {
			if StrSliceContains(values, node.Name) {
				nodes = append(nodes, node)
			}
		} else if name == "id" {
			if StrSliceContains(values, node.UUID) {
				nodes = append(nodes, node)
			}
		} else if name == "primary_ip" {
			if StrSliceContains(values, node.PrimaryIP) {
				nodes = append(nodes, node)
			}
		} else if name == "is_master" {
			if StrSliceContains(values, fmt.Sprintf("%v", node.IsMaster != 0)) {
				nodes = append(nodes, node)
			}
		} else if name == "api_responding" {
			if StrSliceContains(values, fmt.Sprintf("%v", node.APIResponding != 0)) {
				nodes = append(nodes, node)
			}
		} else if name == "cluster_name" {
			if StrSliceContains(values, node.ClusterName) {
				nodes = append(nodes, node)
			}
		} else if name == "cluster_uuid" {
			if StrSliceContains(values, node.ClusterUUID) {
				nodes = append(nodes, node)
			}
		} else if name == "node_pool_name" {
			if StrSliceContains(values, node.NodePoolName) {
				nodes = append(nodes, node)
			}
		} else if name == "node_pool_uuid" {
			if StrSliceContains(values, node.NodePoolUUID) {
				nodes = append(nodes, node)
			}
		} else {
			resp.Diagnostics.AddError("Invalid filter name",
				"Allowed filter names are: name, id, primary_ip, is_master, api_responding, cluster_name, cluster_uuid, node_pool_name, node_pool_uuid")
			return
		}
	}
	var tfNodes []attr.Value
	for _, node := range nodes {
		tfNodesCustVal := datasource_nodes.NodesValue{
			Id:            types.StringValue(node.UUID),
			Name:          types.StringValue(node.Name),
			PrimaryIp:     types.StringValue(node.PrimaryIP),
			IsMaster:      types.BoolValue(node.IsMaster != 0),
			ApiResponding: types.BoolValue(node.APIResponding != 0),
			ClusterName:   types.StringValue(node.ClusterName),
			ClusterUuid:   types.StringValue(node.ClusterUUID),
			NodePoolName:  types.StringValue(node.NodePoolName),
			NodePoolUuid:  types.StringValue(node.NodePoolUUID),
		}
		tfNodesObjVal, diags := tfNodesCustVal.ToObjectValue(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		tfNodesAttrVal, diags := datasource_nodes.NewNodesValue(tfNodesObjVal.AttributeTypes(ctx), tfNodesObjVal.Attributes())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		tfNodes = append(tfNodes, tfNodesAttrVal)
	}

	// data value setting
	state.Id = types.StringValue(data.Id.ValueString())
	nodesListVal, diags := types.ListValue(datasource_nodes.NodesValue{}.Type(ctx), tfNodes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Nodes = nodesListVal

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func StrSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
