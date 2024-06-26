// Code generated by terraform-plugin-framework-generator DO NOT EDIT.

package provider_pf9

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
)

func Pf9ProviderSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"account_url": schema.StringAttribute{
				Required:            true,
				Description:         "Account URL associated with platform9 management control plane",
				MarkdownDescription: "Account URL associated with platform9 management control plane",
			},
			"password": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				Description:         "Password for platform9 management control plane",
				MarkdownDescription: "Password for platform9 management control plane",
			},
			"region": schema.StringAttribute{
				Optional: true,
			},
			"tenant": schema.StringAttribute{
				Optional: true,
			},
			"username": schema.StringAttribute{
				Required:            true,
				Description:         "Username for platform9 management control plane",
				MarkdownDescription: "Username for platform9 management control plane",
			},
		},
	}
}

type Pf9Model struct {
	AccountUrl types.String `tfsdk:"account_url"`
	Password   types.String `tfsdk:"password"`
	Region     types.String `tfsdk:"region"`
	Tenant     types.String `tfsdk:"tenant"`
	Username   types.String `tfsdk:"username"`
}
