# Konform Terraform Provider

With the Konform Terraform Platform9 provider, we enable customers to create and manage their PMK clusters in a cloud-native way. Allowing them to integrate this with other components that they may already be managing with terraform, like AWS, openstack, etc.

## Authenticating with Platform9

Konform requires you to set access credentials to the Platform9 Management Plane before running terraform. All the values can be found by logging into the P9 Console.

DU_FQDN: Navigate to the API Access tab and locate the "API Endpoints" table – there, you will kind the full URL.

DU_USERNAME: Click the top right user icon – the email displayed under your name is your username. If you click on "My Account", you may confirm or update this value.

DU_PASSWORD: The password used to log in to the PMK UI and also may be updated under the "My Account" details.

DU_TENANT: The name of the "Tenant" which you are currently scoped to, via the selected dropdown which is displayed to the left of the user icon.

The following values are used as constants. Please set in the environment running terraform.

```bash
export DU_USERNAME=<Platform9 Username>
export DU_PASSWORD=<Platform9 Password>
export DU_FQDN=<Platform9 DU FQDN>
export DU_TENANT=<Platform9 Tenant Name>
```

## Example Usage

Cluster configuration options should be added to the terraform script. An example .tf using your AWS account as the cloud provider is below. This will create a new cloud provider named "sample_aws_prov" in your Platform9 account. Then it will create a cluster named "cluster_1" using that provider.

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

Initialize the directory `terraform init`.

Format your configuration `terraform fmt`.

Validate your configuration `terraform validate`.

Apply your configuration `terraform apply`.

## Argument Reference

n/a
