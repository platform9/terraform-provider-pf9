# Terraform Provider for Platform9

The provider is built on top of the Platform9 Managed Kubernetes (PMK) platform, which is a managed Kubernetes service that provides a single pane of glass for managing multiple clusters.

## Using the Provider

Refer to the [documentation](https://registry.terraform.io/providers/platform9/pf9/latest/docs) on the Terraform Registry for detailed instructions on how to use this provier. You can also refer to the [docs](./docs/) for documentation of the development version.

## Development Setup

To set up your development environment, follow these steps:
1. Create a new file called `.terraformrc` in your home directory if it doesn't already exist.
2. Add the following `dev_overrides` block to your `.terraformrc` file:
```terraform
provider_installation {
  dev_overrides {
    # Replace the path with the location where the provider binary is installed on your system.
    "platform9/pf9" = "/home/<your-username>/go/bin"
  }
  direct {}
}
```
3. You don't need to run `terraform init` after adding the `dev_overrides` block to the `.terraformrc` file. Terraform automatically uses the development version of the provider when you run `terraform apply` or `terraform plan`.

## Building the Provider

### Requirements

- [Terraform](https://www.terraform.io/downloads.html)
- [Go](https://golang.org/doc/install) to build the provider plugin
- [Visual Studio Code](https://code.visualstudio.com/download) (optional, but recommended)
- [Make](https://www.gnu.org/software/make/) for running the Makefile
- [GoReleaser](https://goreleaser.com/install/) for creating releases

### Development Workflow

```shell
# Build the provider and install its binary in GOBIN path, /home/<your-username>/go/bin
make install

# Add new resource/data source in the `provider_code_spec.json`. Refer existing resource/data-source.
code provider_code_spec.json

# Use the following command to generate corresponding go types
make generate-code

# Scaffold code for a new resource or data source
NAME=newresource make scaffold-rs

# Modify the scaffolded code to implement the resource or data source
code internal/provider/newresource_resource.go

# Add documentation for the new resource or data source, use templates for attributes and examples. Refer existing templates.
code templates/resources/newresource.md.tmpl

# Generate the documentation for terraform registry
make generate
```

### Debugging with Visual Studio Code

1. Set breakpoints and start the debugging session in Visual Studio Code using `launch.json` already included in the repo.
2. Copy the value of the `TF_REATTACH_PROVIDERS` from the *DEBUG CONSOLE* tab in Visual Studio Code.
3. Open a terminal and set the `TF_REATTACH_PROVIDERS` environment variable to the copied value.
4. In the terminal, run `terraform apply` or `terraform plan` to trigger the provider execution and hit the breakpoints.

## Contributing

1. Clone this repository locally.
2. Make any changes you want in your cloned repository, and when you are ready to send those changes to us, push your changes to an upstream branch and [create a pull request](https://help.github.com/articles/creating-a-pull-request/).
3. Once your pull request is created, a reviewer will take responsibility for providing clear, actionable feedback. As the owner of the pull request, it is your responsibility to modify your pull request to address the feedback that has been provided to you by the reviewer(s).
4. After your review has been approved, it will be merged into to the repository.