# Used to manage cluster resource
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
  master_ip                    = "10.149.107.237"
  master_vip_ipv4              = "10.149.107.237"
  master_vip_iface             = "ens3"
  containers_cidr              = "10.20.0.0/16"
  services_cidr                = "10.21.0.0/16"
  mtu_size                     = 1440
  privileged                   = true
  deploy_luigi_operator        = false
  use_hostname                 = false
  interface_detection_method   = "InterfaceName"
  interface_name               = "ens3"
  cpu_manager_policy           = "none"
  topology_manager_policy      = "none"
  calico_ip_ip_mode            = "Always"
  calico_nat_outgoing          = true
  calico_v4_block_size         = "26"
  calico_ipv4_detection_method = "interface=ens3"
  network_plugin               = "calico"
  runtime_config               = ""
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
  monitoring = {
    retention_time = "7d"
  }
  tags = {
    "key1" = "value1"
    "key2" = "value2"
  }
}