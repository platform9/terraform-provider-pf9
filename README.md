# Konform Terraform Provider

Available in the [Terraform Registry](https://registry.terraform.io/providers/platform9/pf9/latest).

[Terraform](https://www.terraform.io/) has been widely regarded by the industry as a leader in the “infrastructure-as-code” space. With konform, we now enable customers to create and manage their PMK clusters with terraform, allowing them to integrate this with other components that they may already be managing with terraform, like AWS, openstack, etc.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 0.12.x

## Using the Provider

See the [Konform Provider documentation](https://registry.terraform.io/providers/hashicorp/hcp/latest/docs) to lean about using the provider.

### Getting Started

Please set the following values in your environment or source them from a script.

```shell
export DU_USERNAME=<Platform9 Username>
export DU_PASSWORD=<Platform9 Password>
export DU_FQDN=<Platform9 DU FQDN>
export DU_TENANT=<Platform9 Tenant Name>
```

Cluster configuration options should be added to the terraform script. An example .tf using your AWS account as the cloud provider is below.

```terraform
terraform {
  required_providers {
    konform = {
      source = "platform9/pf9"
      version = "0.1.4"
    }
  }
}

provider "konform" {}

resource "pf9_aws_cloud_provider" "sample_aws_prov" {
    name                = "sample_aws_provider"
    type                = "aws"
    key                 = "<YOUR_AWS_KEY>"
    secret              = "<YOUR_AWS_SECRET>"
    project_uuid        = "<YOUR_P9_PROJECT_UUID>"
}

resource "pf9_cluster" "cluster_1" {
    project_uuid        = "<YOUR_P9_PROJECT_UUID>"
    allow_workloads_on_master = 0
    ami                 = "ubuntu"
    app_catalog_enabled = 0
    azs                 = [
                            "us-west-2a",
                            "us-west-2b",
                            "us-west-2c",
                            "us-west-2d"
                        ]
    containers_cidr     = "10.20.0.0/16"
    domain_id           = "/hostedzone/DOMAINID"
    external_dns_name   = "auto-generate"
    internal_elb        = false
    is_private          = "false"
    master_flavor       = "t2.medium"
    name                = "tf-sample-1"
    network_plugin      = "flannel"
    node_pool_uuid      = "5c703dc1-6037-44a6-b7f4-1d7fc89cbca6"
    privileged          = 1
    region              = "us-west-2"
    runtime_config      = ""
    service_fqdn        = "auto-generate"
    services_cidr       = "10.21.0.0/16"
    ssh_key             = "<YOUR_AWS_KEY-PAIR_NAME>"
    use_pf9_domain      = "true"
    worker_flavor       = "t2.medium"
    num_masters         = 1
    num_workers         = 3
    enable_cas          = "false"
    masterless          = 1
}
```

With that in place Run `terraform init` to initalize the plugin.

## Contributing

1. Clone this repository locally.
2. Make any changes you want in your cloned repository, and when you are ready to send those changes to us, push your changes to an upstream branch and [create a pull request](https://help.github.com/articles/creating-a-pull-request/).
3. Once your pull request is created, a reviewer will take responsibility for providing clear, actionable feedback. As the owner of the pull request, it is your responsibility to modify your pull request to address the feedback that has been provided to you by the reviewer(s).
4. After your review has been approved, it will be merged into to the repository.
