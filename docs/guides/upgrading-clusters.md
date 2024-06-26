---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "Manage cluster upgrade using Terraform"
subcategory: ""
description: |-
  Terraform provider enables users to upgrade the kubernetes and pmk version.
---

# Cluster version upgrade using Terraform

To upgrade a Terraform-managed PMK cluster, you can specify the target `kube_role_version` in the resource attributes of the `pf9_cluster` block. The Platform9 provider will handle the upgrade process automatically.

If the cluster was created without specifying the `kube_role_version`, it would have been created with the default latest version available at that time. If you want to create a cluster with a specific version, you can specify the `kube_role_version` attribute in the `pf9_cluster` block during creation.

To identify the available upgrade versions, you can check the value of the `upgrade_kube_role_version` attribute of the pf9_cluster resource or data source as shown below.

```terraform
resource "pf9_cluster" "example" {
  name = "example"
  master_nodes = [
    data.pf9_nodes.master.nodes[0].id
  ]
  worker_nodes              = data.pf9_nodes.workers.nodes[*].id
  kube_role_version         = "1.27.10-pmk.96"
  # Uncomment the folloiwng line for upgrade
  # kube_role_version         = "1.28.6-pmk.26"
  allow_workloads_on_master = false
}

data "pf9_cluster" "example" {
  id = "bbbd1c20-3cda-405d-ae4b-d0337fffd6e1"
}

output "upgrade_kube_role_version" {
  value = data.pf9_cluster.example.upgrade_kube_role_version
}
```
