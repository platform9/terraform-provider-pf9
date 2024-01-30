package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = (*pf9Provider)(nil)

func New() func() provider.Provider {
	return func() provider.Provider {
		return &pf9Provider{}
	}
}

type pf9Provider struct{}

func (p *pf9Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {

}

func (p *pf9Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {

}

func (p *pf9Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pf9"
}

func (p *pf9Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *pf9Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}
