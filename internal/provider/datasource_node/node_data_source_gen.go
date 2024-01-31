// Code generated by terraform-plugin-framework-generator DO NOT EDIT.

package datasource_node

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func NodeDataSourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_responding": schema.BoolAttribute{
				Computed:            true,
				Description:         "true indicates the API server on this node is running.",
				MarkdownDescription: "true indicates the API server on this node is running.",
			},
			"cluster_name": schema.StringAttribute{
				Computed:            true,
				Description:         "Name of the cluster the node belongs to",
				MarkdownDescription: "Name of the cluster the node belongs to",
			},
			"cluster_uuid": schema.StringAttribute{
				Computed:            true,
				Description:         "UUID of the cluster the node belongs to",
				MarkdownDescription: "UUID of the cluster the node belongs to",
			},
			"id": schema.StringAttribute{
				Required:            true,
				Description:         "UUID of the cluster",
				MarkdownDescription: "UUID of the cluster",
			},
			"is_master": schema.BoolAttribute{
				Computed:            true,
				Description:         "true if this node is a master of a cluster.",
				MarkdownDescription: "true if this node is a master of a cluster.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Description:         "Host name of the node",
				MarkdownDescription: "Host name of the node",
			},
			"node_pool_name": schema.StringAttribute{
				Computed:            true,
				Description:         "Name of the node pool, the node belongs to",
				MarkdownDescription: "Name of the node pool, the node belongs to",
			},
			"node_pool_uuid": schema.StringAttribute{
				Computed:            true,
				Description:         "UUID of the node pool, the node belongs to",
				MarkdownDescription: "UUID of the node pool, the node belongs to",
			},
			"primary_ip": schema.StringAttribute{
				Computed:            true,
				Description:         "IP address of the node",
				MarkdownDescription: "IP address of the node",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				Description:         "Status of the node. States include “ok”, ”converging”, “failed”. These states indicate the current state of kubernetes setup on the host.",
				MarkdownDescription: "Status of the node. States include “ok”, ”converging”, “failed”. These states indicate the current state of kubernetes setup on the host.",
			},
		},
	}
}

type NodeModel struct {
	ApiResponding types.Bool   `tfsdk:"api_responding"`
	ClusterName   types.String `tfsdk:"cluster_name"`
	ClusterUuid   types.String `tfsdk:"cluster_uuid"`
	Id            types.String `tfsdk:"id"`
	IsMaster      types.Bool   `tfsdk:"is_master"`
	Name          types.String `tfsdk:"name"`
	NodePoolName  types.String `tfsdk:"node_pool_name"`
	NodePoolUuid  types.String `tfsdk:"node_pool_uuid"`
	PrimaryIp     types.String `tfsdk:"primary_ip"`
	Status        types.String `tfsdk:"status"`
}
