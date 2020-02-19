package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/platform9/konform/pf9"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: pf9.Provider})
}
