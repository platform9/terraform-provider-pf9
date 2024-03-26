package provider

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_clusters"

	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
)

var _ datasource.DataSource = (*clustersDataSource)(nil)

func NewClustersDataSource() datasource.DataSource {
	return &clustersDataSource{}
}

type clustersDataSource struct {
	client *pmk.HTTPClient
}

func (d *clustersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clusters"
}

func (d *clustersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_clusters.ClustersDataSourceSchema(ctx)
}

func (d *clustersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_clusters.ClustersModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var filters []datasource_clusters.FiltersValue
	resp.Diagnostics.Append(data.Filters.ElementsAs(ctx, &filters, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	clusters, err := d.client.Qbert().ListClusters(qbert.ListOptions{
		All: true,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to list clusters", err.Error())
		return
	}
	for _, filter := range filters {
		attribName := filter.Name.ValueString()
		var values []string
		resp.Diagnostics.Append(filter.Values.ElementsAs(ctx, &values, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var regexes []string
		if len(values) == 0 {
			resp.Diagnostics.Append(filter.Regexes.ElementsAs(ctx, &regexes, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		switch {
		case attribName == "name":
			clusters = filterClusters(clusters, attribName, values, regexes)
		case attribName == "tenant_id": //TODO: Ask PMK team/Miguel what do we mean by tenant
			clusters = filterClusters(clusters, attribName, values, regexes)
		case strings.HasPrefix("tags:", attribName):
			clusters = filterClustersByTags(clusters, attribName, values)
		default:
			resp.Diagnostics.AddError("Unknown filter", "Unknown filter: "+attribName)
			return
		}
	}

	var clusterIDs []string
	for _, cluster := range clusters {
		clusterIDs = append(clusterIDs, cluster.UUID)
	}

	var diags diag.Diagnostics
	data.ClusterIds, diags = types.ListValueFrom(ctx, basetypes.StringType{}, clusterIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func filterClusters(clusters []qbert.Cluster, attribName string, values, regexes []string) []qbert.Cluster {
	var filteredClusters []qbert.Cluster
	for _, cluster := range clusters {
		var valueFromCluster string
		switch attribName {
		case "name":
			valueFromCluster = cluster.Name
		case "tenant_id":
			valueFromCluster = cluster.ProjectId
		}
		for _, value := range values {
			if valueFromCluster == value {
				filteredClusters = append(filteredClusters, cluster)
			}
		}
		for _, regex := range regexes {
			if ok, _ := regexp.Match(regex, []byte(valueFromCluster)); ok {
				filteredClusters = append(filteredClusters, cluster)
			}
		}
	}
	return filteredClusters
}

func filterClustersByTags(clusters []qbert.Cluster, attribName string, values []string) []qbert.Cluster {
	var filteredClusters []qbert.Cluster
	// tagKey = "environment"
	tagKey := strings.TrimPrefix(attribName, "tags:")
	if tagKey == "" {
		return filteredClusters
	}
	for _, cluster := range clusters {
		// values = ["production", "development"]
		for _, value := range values {
			if cluster.Tags[tagKey] == value {
				// this cluster has tag "environment" with value "production" or "development"
				filteredClusters = append(filteredClusters, cluster)
				break
			}
		}
	}
	return filteredClusters
}
