//go:build tools
// +build tools

package tools

// What is this? https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md

import (
	_ "github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework"
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
