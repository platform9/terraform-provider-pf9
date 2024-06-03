package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/platform9/terraform-provider-pf9/internal/provider"
)

// Format example terraform files
//go:generate terraform fmt -recursive ./examples/

// TODO: How to ensure version of tools(tfplugondocs and tfplugin-framework) should be same as that of go.mod or
// tools.go. IOW, how can we prevent this from running latest version of tools?

// Install the codegen tool
//go:generate go install github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework

// Generate resource, datasource, provider schema from provider_code_spec.json using codegen tool
//go:generate tfplugingen-framework generate all --input ./provider_code_spec.json --output internal/provider

// Run the docs generation tool
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var (
	// these will be set by the goreleaser configuration
	version string = "dev"
	// https://goreleaser.com/cookbooks/using-main.version/
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/platform9/pf9",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
