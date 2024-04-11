---
page_title: "Create Cluster with Custom Settings"
subcategory: ""
description: |-
  The terraform provider facilitates the creation of clusters with different configurations, such as multi-master and single-node setups..
---

# Create Cluster with Custom Settings

This guide provides examples of provisioning clusters with custom configurations using PF9 Terraform provider. The Terraform provider supports most of the attributes that are available through the PF9 [PMK APIs](https://platform9.com/docs/qbert/ref#postcreates-a-cluster-using-auto-deploy-or-manual-mode). However, for the sake of simplicity, only the essential attributes are included in the examples. All supported attributes are documented in the cluster resource [documentation](../docs/resources/cluster.md).

## Single Node Cluster with Default Settings

This example demonstrates how to create a single-node cluster with default settings.

```terraform
terraform {
  required_providers {
    pf9 = {
      source = "platform9/pf9"
    }
  }
}

provider "pf9" {
  # The environment variable DU_PASSWORD must be set
  du_fqdn     = "<FQDN>"
  du_username = "<Username>"
}

resource "pf9_cluster" "example" {
  name = "single-node-cluster"
  # UUIDs of the nodes that are connected to the PF9 PMK control plane
  # and not in Converging state and not assigned to another cluster.
  master_nodes = [
    "2c5f75a1-5fb3-4d18-b9df-b6313d483961"
  ]
  worker_nodes              = []
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = false
  }
}
```

## Single Master Cluster

```terraform
terraform {
  required_providers {
    pf9 = {
      source = "platform9/pf9"
    }
  }
}

provider "pf9" {
  du_fqdn     = "<FQDN>"
  du_username = "<Username>"
}

resource "pf9_cluster" "example" {
  name = "single-master-cluster"
  master_nodes = [
    "2c5f75a1-5fb3-4d18-b9df-b6313d483961"
  ]
  worker_nodes = [
    "a51a04b1-1602-4dfa-9490-88bef5d3aa2c"
  ]
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = false
  }
}
```

## Multi Master Cluster

To create a [Multi-Master cluster](https://platform9.com/docs/kubernetes/multimaster-architecture-platform9-managed-kubernetes) `master_vip_ipv4` and `master_vip_iface` attributes are required.

```terraform
terraform {
  required_providers {
    pf9 = {
      source = "platform9/pf9"
    }
  }
}

provider "pf9" {
  du_fqdn     = "<FQDN>"
  du_username = "<Username>"
}

resource "pf9_cluster" "example" {
  name = "multi-master-cluster"
  master_nodes = [
    "dfad3588-aba1-4e46-b2db-673c69faf63d"
  ]
  allow_workloads_on_master = false
  worker_nodes              = [
    "359cce4a-1839-4b82-8533-8f2761079cd8"
  ]
  master_vip_ipv4   = "10.149.107.237"
  master_vip_iface  = "ens3"
  # If not set kube role version defaults to the latest
  kube_role_version = "1.26.10-pmk.164"
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
}
```

Automating Cluster Creation
---------------------------

```terraform
terraform {
  required_providers {
    pf9 = {
      source = "platform9/pf9"
    }
  }
}

provider "pf9" {
  du_fqdn     = "<FQDN>"
  du_username = "<Username>"
}

data "pf9_nodes" "masters" {
  filter = {
    name   = "primary_ip"
    values = ["10.149.107.181", "10.149.107.182", "10.149.107.183"]
  }
}

data "pf9_nodes" "workers" {
  filter = {
    name   = "primary_ip"
    values = ["10.149.106.173", "10.149.106.174", "10.149.106.175", "10.149.106.176", "10.149.106.177"]
  }
}

resource "pf9_cluster" "example" {
  name                      = "tf-cluster-50"
  master_nodes              = data.pf9_nodes.masters.nodes[*].id
  worker_nodes              = data.pf9_nodes.workers.nodes[*].id
  allow_workloads_on_master = false
  master_vip_ipv4           = data.pf9_nodes.masters.nodes[0].primary_ip
  master_vip_iface          = "ens3"
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
}

data "pf9_cluster" "example" {
  id = resource.pf9_cluster.example.id
}

output "kubeconfig" {
  value = data.pf9_cluster.example.kubeconfig
}
```
