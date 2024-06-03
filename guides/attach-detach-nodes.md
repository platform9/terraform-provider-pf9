---
page_title: "Using PF9 Terraform Provider to Attach and Detach Nodes"
subcategory: ""
description: |-
  The Platform9 PMK terraform provider provides easier solution to attach and detach nodes.
---

# Attach and Detach Nodes

The PF9 Provider allows you to attach and detach nodes to and from your Platform9 Managed Kubernetes (PMK) clusters as master or worker nodes, giving you flexibility in managing your cluster's infrastructure.

## Finding node IDs

The node id can be obtained using the nodes data source. The nodes data source supports `filter` attribute which accepts `name` and `values` attributes.

The `name` attribute supports the following values:

- `name`
- `id`
- `primary_ip`
- `is_master`
- `api_responding`
- `cluster_name`
- `cluster_uuid`
- `node_pool_name`
- `node_pool_uuid`
- `status`

## Attaching Nodes

To attach nodes to your PMK cluster as master or worker nodes, you can use the nodes data source to filter for the nodes you want to attach and obtain their IDs programmatically. For example:


```terraform
data "pf9_nodes" "master" {
  filter = {
    name = "primary_ip"
    values = ["192.168.1.25"]
  }
}

data "pf9_nodes" "workers" {
  filter = {
    name = "primary_ip"
    values = ["192.168.1.26", "192.168.1.27", "192.168.1.28"]
  }
}

resource "pf9_cluster" "example" {
  name = "example"
  master_nodes = [
    data.pf9_nodes.master.nodes[0].id
  ]
  allow_workloads_on_master = false
  worker_nodes = data.pf9_nodes.workers.nodes[*].id
  tags = {
    "key1" = "value1"
  }
}
```

## Detaching Nodes

To detach nodes from your PMK cluster, simply remove their IDs from the `master_nodes` or `worker_nodes` attributes in the pf9_cluster resource configuration. For example, to detach a node with ID "2bfbc40e-1d72-4bfc-a46b-56b674862cc7" from the worker nodes:

```terraform
# Used to manage cluster resource
resource "pf9_cluster" "example" {
  name = "example"
  master_nodes = [
    "17f9b392-67bb-43b9-b0b7-3b5821f683a6",
  ]
  allow_workloads_on_master = false
  worker_nodes = [
    # "2bfbc40e-1d72-4bfc-a46b-56b674862cc7", Removed
    "bbbd1c20-3cda-405d-ae4b-d0337fffd6e1"
  ]
  # .. other attributes
}
```

## Changing Node Roles

To change the role of a node from master to worker or vice versa, simply move its ID from `master_nodes` to `worker_nodes` or vice versa in the `pf9_cluster` resource configuration.

## Identifying Nodes Available to Attach

Often, it becomes necessary to identify the nodes that are available to attach to a cluster. This can be done by filtering the `hosts` that are connected to the PF9 managed control plane, and then further filtering the output based on the `status` attribute and `cluster_name` to find the nodes that are not yet part of a cluster.

The following example shows how to find the node IDs of the nodes that are available to be attached to a cluster:

```terraform
# Filter hosts connected to PMK
data "pf9_hosts" "connected" {
  filters = [
    {
      name   = "responding"
      values = ["true"]
    }
  ]
}

# Filter nodes that are available to be added to a cluster
data "pf9_nodes" "available" {
  filters = [
    {
      name   = "id",
      values = data.pf9_hosts.connected.host_ids
    },
    {
      name   = "status"
      values = ["ok"]
    },
    {
      name   = "cluster_name"
      values = [""]
    }
  ]
}

output "example" {
  value = data.pf9_nodes.available.node_ids
}
```