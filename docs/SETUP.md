# konform

## Terraform provider for PMK

Konform allows you to create and manage your PMK clusters using terraform.

### Getting Started

Konform requires Go v1.13+ to build.

Build the PMK terraform plugin.

```shell
go build -o terraform-provider-pf9
```

The binary needs to be of the form `terraform-provider-*`. If built correctly you should see a message like this when you run it directly:

```shell
[~/p/s/konform] : ./terraform-provider-pf9
This binary is a plugin. These are not meant to be executed directly.
Please execute the program that consumes these plugins, which will
load any plugins automatically
```

Terraform looks for the plugin in the local directory as well as under `$HOME/.terraform.d/plugins `. Once built, you may side load this executables onto any host that runs terraform by copying it into the directory given above. Please refer to the [terraform documentation](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins) for more details.
