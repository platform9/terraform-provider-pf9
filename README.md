# Konform Terraform Provider

Available in the [Terraform Registry](https://registry.terraform.io/namespaces/platform9).

[Terraform](https://www.terraform.io/) has been widely regarded by the industry as a leader in the “infrastructure-as-code” space. With konform, we now enable customers to create and manage their PMK clusters with terraform, allowing them to integrate this with other components that they may already be managing with terraform, like AWS, openstack, etc.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 0.12.x

## Using the Provider

See the [Konform Provider documentation](https://registry.terraform.io/providers/platform9/pf9/latest/docs) to lean about using the provider.

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
    pf9 = {
      source = "platform9/pf9"
      version = "0.1.5"
    }
  }
}

provider "pf9" {}

resource "pf9_aws_cloud_provider" "sample_aws_prov" {
    name                = "sample_aws_provider"
    type                = "aws"
    key                 = "<YOUR_AWS_KEY>"
    secret              = "<YOUR_AWS_SECRET>"
    project_uuid        = "<YOUR_P9_PROJECT_UUID>"
}

resource "pf9_cluster" "cluster_1" {
  project_uuid        = "<YOUR_P9_PROJECT_UUID>"
  cloud_provider_uuid = ""
  name                = "some-memorable-cluster-name"
  ami                 = "ubuntu"
  azs                 = ["us-east-1b"]
  region              = "us-east-1"
  containers_cidr     = "10.20.0.0/16"
  services_cidr       = "10.21.0.0/16"
  worker_flavor       = "t2.medium"
  master_flavor       = "t2.medium"
  ssh_key             = "<MY_AWS_KEY_PAIR>"
  num_masters         = 1
  num_workers         = 1
}
```

With that in place `terraform init` to initalize the plugin.
Then `terraform apply` to run.

## Contributing

1. Clone this repository locally.
2. Make any changes you want in your cloned repository, and when you are ready to send those changes to us, push your changes to an upstream branch and [create a pull request](https://help.github.com/articles/creating-a-pull-request/).
3. Once your pull request is created, a reviewer will take responsibility for providing clear, actionable feedback. As the owner of the pull request, it is your responsibility to modify your pull request to address the feedback that has been provided to you by the reviewer(s).
4. After your review has been approved, it will be merged into to the repository.
