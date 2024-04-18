package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_nodes"
)

var _ datasource.DataSource = (*nodesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*nodesDataSource)(nil)

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
	var data datasource_nodes.NodesModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	authInfo, err := d.client.Authenticator().Auth(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID
	filteredNodes, err := d.client.Qbert().ListNodes(projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list nodes", err.Error())
		return
	}

	if !data.Filters.IsNull() {
		var filtersValue []datasource_nodes.FiltersValue
		diags := data.Filters.ElementsAs(ctx, &filtersValue, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, filter := range filtersValue {
			var values []string
			name := filter.Name.ValueString()
			resp.Diagnostics.Append(filter.Values.ElementsAs(ctx, &values, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			var regexes []string
			resp.Diagnostics.Append(filter.Regexes.ElementsAs(ctx, &regexes, false)...)
			filteredNodes, err = filterNodes(filteredNodes, name, values, regexes)
			if err != nil {
				resp.Diagnostics.AddError("Failed to filter nodes", err.Error())
				return
			}
		}
	} else if !data.Filter.IsNull() {
		var values []string
		name := data.Filter.Name.ValueString()
		resp.Diagnostics.Append(data.Filter.Values.ElementsAs(ctx, &values, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		filteredNodes, err = filterNodes(filteredNodes, name, values, nil)
		if err != nil {
			resp.Diagnostics.AddError("Failed to filter nodes", err.Error())
			return
		}
	}

	nodeIDs := make([]string, len(filteredNodes))
	nodesValues := make([]datasource_nodes.NodesValue, len(filteredNodes))
	for i, node := range filteredNodes {
		nodesObjValue, diags := datasource_nodes.NodesValue{
			Id:            types.StringValue(node.UUID),
			Name:          types.StringValue(node.Name),
			PrimaryIp:     types.StringValue(node.PrimaryIP),
			IsMaster:      types.BoolValue(node.IsMaster != 0),
			ApiResponding: types.BoolValue(node.APIResponding != 0),
			ClusterName:   types.StringValue(node.ClusterName),
			ClusterUuid:   types.StringValue(node.ClusterUUID),
			NodePoolName:  types.StringValue(node.NodePoolName),
			NodePoolUuid:  types.StringValue(node.NodePoolUUID),
			Status:        types.StringValue(node.Status),
		}.ToObjectValue(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		nodesValue, diags := datasource_nodes.NewNodesValue(nodesObjValue.AttributeTypes(ctx), nodesObjValue.Attributes())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		nodesValues[i] = nodesValue
		nodeIDs[i] = node.UUID
	}

	nodesListVal, diags := types.ListValueFrom(ctx, datasource_nodes.NodesValue{}.Type(ctx), nodesValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Nodes = nodesListVal
	nodeIDsVal, diags := types.ListValueFrom(ctx, types.StringType, nodeIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.NodeIds = nodeIDsVal

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func filterNodes(nodes []qbert.Node, filterName string, filterValues []string, filterRegexes []string) ([]qbert.Node, error) {
	var filteredNodes []qbert.Node
	for _, node := range nodes {
		if filterName == "name" {
			if StrSliceContains(filterValues, node.Name) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.Name) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "status" {
			if StrSliceContains(filterValues, node.Status) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.Status) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "id" {
			if StrSliceContains(filterValues, node.UUID) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.UUID) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "primary_ip" {
			if StrSliceContains(filterValues, node.PrimaryIP) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.PrimaryIP) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "is_master" {
			if StrSliceContains(filterValues, fmt.Sprintf("%v", node.IsMaster != 0)) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "api_responding" {
			if StrSliceContains(filterValues, fmt.Sprintf("%v", node.APIResponding != 0)) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "cluster_name" {
			if StrSliceContains(filterValues, node.ClusterName) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.ClusterName) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "cluster_uuid" {
			if StrSliceContains(filterValues, node.ClusterUUID) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.ClusterUUID) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "node_pool_name" {
			if StrSliceContains(filterValues, node.NodePoolName) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.NodePoolName) {
				filteredNodes = append(filteredNodes, node)
			}
		} else if filterName == "node_pool_uuid" {
			if StrSliceContains(filterValues, node.NodePoolUUID) {
				filteredNodes = append(filteredNodes, node)
			}
			if RegexMatches(filterRegexes, node.NodePoolUUID) {
				filteredNodes = append(filteredNodes, node)
			}
		} else {
			return nil, errors.New("invalid filter name")
		}
	}
	return filteredNodes, nil
}
