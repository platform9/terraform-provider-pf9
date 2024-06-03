package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_host"
)

var _ datasource.DataSource = (*hostDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*hostDataSource)(nil)

func NewHostDataSource() datasource.DataSource {
	return &hostDataSource{}
}

type hostDataSource struct {
	client *pmk.HTTPClient
}

func (d *hostDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (d *hostDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_host.HostDataSourceSchema(ctx)
}

func (d *hostDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *hostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_host.HostModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	hostID := data.Id.ValueString()
	_, err := d.client.Authenticator().Auth(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	host, err := d.client.Resmgr().GetHost(ctx, hostID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get host", err.Error())
		return
	}
	data.Arch = types.StringValue(host.Info.Arch)
	data.Hostname = types.StringValue(host.Info.Hostname)
	extensions, err := host.ReadExtensions()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read extensions", err.Error())
		return
	}
	ifaces := []datasource_host.InterfacesValue{}
	for key, val := range extensions.Interfaces.Data.InterfaceIP {
		ifaceObjVal, diags := datasource_host.InterfacesValue{
			Name: types.StringValue(key),
			Ip:   types.StringValue(val),
		}.ToObjectValue(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		ifaceVal, diags := datasource_host.NewInterfacesValue(ifaceObjVal.AttributeTypes(ctx), ifaceObjVal.Attributes())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		ifaces = append(ifaces, ifaceVal)
	}
	var diags diag.Diagnostics
	data.Interfaces, diags = types.ListValueFrom(ctx, datasource_host.InterfacesValue{}.Type(ctx), ifaces)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if host.Info.LastResponseTime != nil {
		if strLastRespTime, ok := host.Info.LastResponseTime.(string); ok {
			data.LastResponseTime = types.StringValue(strLastRespTime)
		} else {
			data.LastResponseTime = types.StringNull()
		}
	} else {
		data.LastResponseTime = types.StringNull()
	}
	data.Message = types.StringValue(host.Message)
	data.OsFamily = types.StringValue(host.Info.OSFamily)
	data.OsInfo = types.StringValue(host.Info.OSInfo)
	data.Responding = types.BoolValue(host.Info.Responding)
	data.RoleStatus = types.StringValue(host.RoleStatus)
	data.Roles, diags = types.ListValueFrom(ctx, types.StringType, host.Roles)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
