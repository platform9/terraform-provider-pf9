variable "master_ip" {
  description = "The primary IP address of the master node"
  type = string
}

variable "worker1_hostname" {
  description = "The hostname of the worker node"
  type = string
}

# find node with hostname worker1
data "pf9_nodes" "workers" {
  filter = {
    name   = "name"
    values = [var.worker1_hostname]
  }
}

# find node with primary ip address
data "pf9_nodes" "master" {
  filter = {
    name   = "primary_ip"
    values = [var.master_ip]
  }
}

# Retrieve the interface name associated with the master IP
data "pf9_host" "master" {
  id = data.pf9_nodes.master.nodes[0].id
}

locals {
  iface_name = try(
    [for iface in data.pf9_host.master.interfaces : iface.name if iface.ip == var.master_ip][0],
    null
  )
}

resource "pf9_cluster" "example" {
  name                         = "example"
  master_nodes                 = toset(data.pf9_nodes.master.nodes[*].id)
  worker_nodes                 = toset(data.pf9_nodes.workers.nodes[*].id)
  allow_workloads_on_master    = false
  master_vip_ipv4              = data.pf9_nodes.workers.nodes[0].primary_ip
  master_vip_iface             = local.iface_name
  containers_cidr              = "10.20.0.0/16"
  services_cidr                = "10.21.0.0/16"
  interface_detection_method   = "InterfaceName"
  interface_name               = local.iface_name
  network_plugin               = "calico"
  calico_ipv4_detection_method = "interface=${local.iface_name}"
  etcd_backup = {
    daily = {
      backup_time = "02:00"
    }
  }
  tags = {
    "key1" = "value1"
  }
  depends_on = [ data.pf9_host.master, data.pf9_nodes.workers ]
}
