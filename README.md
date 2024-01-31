# Terraform Provider for Platform9

The Platform9 Provider is a solution for creating and managing multiple clusters, attaching and detaching nodes, and managing the lifecycle of the clusters. The provider is built on top of the Platform9 Managed Kubernetes (PMK) platform, which is a managed Kubernetes service that provides a single pane of glass for managing multiple clusters.

## Using the Provider

Refer to the [documentation](https://registry.terraform.io/providers/platform9/pf9/latest/docs) on the Terraform Registry for detailed instructions on how to use this provier. You can also refer to the [docs](./docs/index.md) for documentation of the development version.

## Development Setup

Terraform allows you to use local provider builds by setting a `dev_overrides` block in a configuration file called `~/.terraformrc`. First, find the GOBIN path where Go installs your binaries. Your path may vary depending on how your Go environment variables are configured.

```console
$ go env GOBIN
/home/<Username>/go/bin
```

If the `GOBIN` go environment variable is not set, use the default path, `/home/<Username>/go/bin`. Create a new file called `.terraformrc` in your home directory, then add the `dev_overrides` block below.

```terraform
provider_installation {
  dev_overrides {
    "platform9/pf9" = "/home/<username>>/go/bin"
  }
  direct {}
}
```

After this follow the following steps in vscode:

0. Open [launch.json](.vscode/launch.json) add necessary environment variable values. DO NOT COMMIT THIS FILE!
0. Add breakpoints and launch Debugging configuration.
0. On Debug Console in VS Code, you should see instruction to export an environment variable
0. Export the variable `TF_REATTACH_PROVIDERS` with the value printed on `Debug Console`.
0. Run `terraform apply --auto-approve` on `main.tf`


## Contributing

1. Clone this repository locally.
2. Make any changes you want in your cloned repository, and when you are ready to send those changes to us, push your changes to an upstream branch and [create a pull request](https://help.github.com/articles/creating-a-pull-request/).
3. Once your pull request is created, a reviewer will take responsibility for providing clear, actionable feedback. As the owner of the pull request, it is your responsibility to modify your pull request to address the feedback that has been provided to you by the reviewer(s).
4. After your review has been approved, it will be merged into to the repository.
