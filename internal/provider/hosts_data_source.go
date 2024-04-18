package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/resmgr"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_hosts"
)

var _ datasource.DataSource = (*hostsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*hostsDataSource)(nil)

func NewHostsDataSource() datasource.DataSource {
	return &hostsDataSource{}
}

type hostsDataSource struct {
	client *pmk.HTTPClient
}

func (d *hostsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hosts"
}

func (d *hostsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_hosts.HostsDataSourceSchema(ctx)
}

func (d *hostsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *hostsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_hosts.HostsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	_, err := d.client.Authenticator().Auth(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	hosts, err := d.client.Resmgr().ListHosts(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list hosts", err.Error())
		return
	}
	if !data.Filters.IsNull() {
		var filterValues []datasource_hosts.FiltersValue
		diags := data.Filters.ElementsAs(ctx, &filterValues, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, filter := range filterValues {
			var values []string
			name := filter.Name.ValueString()
			resp.Diagnostics.Append(filter.Values.ElementsAs(ctx, &values, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			var regexes []string
			resp.Diagnostics.Append(filter.Regexes.ElementsAs(ctx, &regexes, false)...)
			hosts, err = filterHosts(hosts, name, values, regexes)
			if err != nil {
				resp.Diagnostics.AddError("Failed to filter nodes", err.Error())
				return
			}
		}
	}
	hostIDs := make([]string, len(hosts))
	for i, host := range hosts {
		hostIDs[i] = host.ID
	}
	hostIDsValue, diags := types.ListValueFrom(ctx, types.StringType, hostIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.HostIds = hostIDsValue

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func filterHosts(hosts []resmgr.Host, name string, values []string, regexes []string) ([]resmgr.Host, error) {
	var filteredHosts []resmgr.Host
	for _, host := range hosts {
		switch name {
		case "id":
			if StrSliceContains(values, host.ID) {
				filteredHosts = append(filteredHosts, host)
			}
			if RegexMatches(regexes, host.ID) {
				filteredHosts = append(filteredHosts, host)
			}
		case "hostname":
			if StrSliceContains(values, host.Info.Hostname) {
				filteredHosts = append(filteredHosts, host)
			}
			if RegexMatches(regexes, host.Info.Hostname) {
				filteredHosts = append(filteredHosts, host)
			}
		case "os_family":
			if StrSliceContains(values, host.Info.OSFamily) {
				filteredHosts = append(filteredHosts, host)
			}
			if RegexMatches(regexes, host.Info.OSFamily) {
				filteredHosts = append(filteredHosts, host)
			}
		case "arch":
			if StrSliceContains(values, host.Info.Arch) {
				filteredHosts = append(filteredHosts, host)
			}
			if RegexMatches(regexes, host.Info.Arch) {
				filteredHosts = append(filteredHosts, host)
			}
		case "os_info":
			if StrSliceContains(values, host.Info.OSInfo) {
				filteredHosts = append(filteredHosts, host)
			}
			if RegexMatches(regexes, host.Info.OSInfo) {
				filteredHosts = append(filteredHosts, host)
			}
		case "message":
			if StrSliceContains(values, host.Message) {
				filteredHosts = append(filteredHosts, host)
			}
			if RegexMatches(regexes, host.Message) {
				filteredHosts = append(filteredHosts, host)
			}
		case "role_status":
			if StrSliceContains(values, host.RoleStatus) {
				filteredHosts = append(filteredHosts, host)
			}
		case "responding":
			if StrSliceContains(values, fmt.Sprintf("%v", host.Info.Responding)) {
				filteredHosts = append(filteredHosts, host)
			}
		case "roles":
			for _, role := range host.Roles {
				if StrSliceContains(values, role) {
					filteredHosts = append(filteredHosts, host)
					break
				}
			}
		default:
			return nil, fmt.Errorf("unknown filter: %s", name)
		}
	}
	return filteredHosts, nil
}
