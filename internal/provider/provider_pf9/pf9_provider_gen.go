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
			"du_fqdn": schema.StringAttribute{
				Optional:            true,
				Description:         "DU FQDN",
				MarkdownDescription: "DU FQDN",
			},
			"du_region": schema.StringAttribute{
				Optional:            true,
				Description:         "Region",
				MarkdownDescription: "Region",
			},
			"du_tenant": schema.StringAttribute{
				Optional:            true,
				Description:         "Tenant",
				MarkdownDescription: "Tenant",
			},
			"du_username": schema.StringAttribute{
				Optional:            true,
				Description:         "Username",
				MarkdownDescription: "Username",
			},
		},
	}
}

type Pf9Model struct {
	DuFqdn     types.String `tfsdk:"du_fqdn"`
	DuRegion   types.String `tfsdk:"du_region"`
	DuTenant   types.String `tfsdk:"du_tenant"`
	DuUsername types.String `tfsdk:"du_username"`
}