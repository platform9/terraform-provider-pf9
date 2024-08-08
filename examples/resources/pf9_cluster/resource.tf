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
  master_vip_ipv4              = "x.x.x.x"
  master_vip_iface             = "ens3"
  containers_cidr              = "10.20.0.0/16"
  services_cidr                = "10.21.0.0/16"
  interface_detection_method   = "InterfaceName"
  interface_name               = "ens3"
  network_plugin               = "calico"
  calico_ipv4_detection_method = "interface=ens3"
  etcd_backup = {
    daily = {
      backup_time = "02:00"
    }
  }
  tags = {
    "key1" = "value1"
  }
  addons = {
    "coredns" = {
      enabled = true
      params = {
        "CoresPerReplica"              = "257"
        "MaxReplicas"                  = "101"
        "MinReplicas"                  = "2"
        "NodesPerReplica"              = "17"
        "PollPeriodSecs"               = "301"
        "dnsDomain"                    = "cluster.local"
        "dnsMemoryLimit"               = "170Mi"
      }
      version = "1.11.1"
    },
    "kubernetes-dashboard" = {
      enabled = true
      params  = {}
    },
    "metallb" = {
      enabled = false
    },
    "metrics-server" = {
      enabled = true
      params = {
        "metricsCpuLimit"    = "100m"
        "metricsMemoryLimit" = "300Mi"
      }
      version = "0.6.4"
    }
  }
}
