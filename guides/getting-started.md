---
page_title: "Getting Started with the PF9 Provider"
subcategory: ""
description: |-
  The Platform9 PMK terraform provider offers a streamlined solution for creating and managing kubernetes clusters.
---

# Getting Started with the PF9 Provider

Platform9 Managed Kubernetes (PMK) is a SaaS managed Kubernetes offering that makes it effortless run Kubernetes on any infrastructure. Head over to the [Platform9 documentation](https://platform9.com/docs/kubernetes) to learn more about PMK.

This guide will help you get started with using the provider to manage your clusters on the Platform9 Managed Kubernetes (PMK) platform.

## Prerequisites

Before you begin, ensure you have the following:

1. Platform9 PMK account credentials (username, password and FQDN). It can be obtained by creating a new account at [Platform9](https://platform9.com/).
2. Nodes connected with the Platform9 PMK platform.
3. Terraform installed on your machine that has access to the internet.

## Onboarding Nodes

Before you can create a cluster, you need to onboard nodes to the Platform9 PMK platform. You can do this using Platform9 CLI. SSH into the node that you want to connect with the Platform9 PMK platform. The `prep-node` command onboards a node. It installs platform9 packages on the host. After completion of this command, the node is available to be managed on the Platform9 control plane.

```bash
bash <(curl -sL https://pmkft-assets.s3-us-west-1.amazonaws.com/pf9ctl_setup) 
pf9ctl prep-node
```

For more information on how to use `pf9ctl`, refer to its [documentation](https://github.com/platform9/pf9ctl/).

## Authentication

To authenticate terraform provider with the PF9 Provider, set the following environment variables:

```bash
# Required
export DU_FQDN="FQDN"
export DU_USERNAME="username"
export DU_PASSWORD="Password" # use single quotes if password contains special characters

# Optional, with default values as shown below
export DU_REGION=RegionOne
export DU_TENANT=service
```

These environment variables will be used by the provider to authenticate with the Platform9 PMK platform. Although it is possible to set these variables in the provider configuration, it is recommended to use environment variables to avoid exposing sensitive information in your configuration files. For security reasons, it is recommended to use a secrets management tool to store and manage these credentials.

## Example Usage

Declare the provider in your configuration in a file named `main.tf`.

```terraform
terraform {
  required_providers {
    pf9 = {
      source = "platform9/pf9"
    }
  }
}

provider "pf9" {}
```

## Create Your First Cluster

Now after declaring the provider, you can create your first cluster. Add the following code to the `main.tf` file, created earlier. Feel free to modify the values as per your requirements. Obtaining node IDs can be done using the Platform9 UI or using nodes data source provided by the provider.

```terraform
resource "pf9_cluster" "example" {
  name = "example"
  master_nodes = [
    "17f9b392-67bb-43b9-b0b7-3b5821f683a6",
    "7f5aa992-0abe-40a0-9bf9-6a06ebb9ccfd",
    "a17fa56d-722b-4f10-8b50-ffa5a4bed36e"
  ]
  allow_workloads_on_master = false
  worker_nodes = [
    "2bfbc40e-1d72-4bfc-a46b-56b674862cc7",
    "bbbd1c20-3cda-405d-ae4b-d0337fffd6e1"
  ]
  master_vip_ipv4           = "10.149.107.237"
  etcd_backup = {
    daily = {
      backup_time = "02:00"
    }
  }
  tags = {
    "key1" = "value1"
  }
}
```

## Apply the Configuration

Run `terraform init` to initialize the provider and download the necessary plugins. Then run `terraform apply` to create the cluster.
  
```bash
terraform init
terraform apply
```

## Modify the Configuration

With the provider, you can modify the configuration of the cluster. For example, you can add or remove nodes, change the network configuration, or modify the tags. After making the changes, run `terraform apply` to apply the changes.

## Destroy the Cluster

When you no longer need the cluster, you can destroy it using `terraform destroy`.

```bash
terraform destroy
```