---
page_title: "Manage cluster addons using Terraform"
subcategory: ""
description: |-
  Terraform provider allows enable, disable or upgrade the cluster addon in declarative manner.
---

## Manage Cluster addons using Terraform

### Overview

The Terraform provider facilitates the declarative management of cluster addons. This guide is designed to illustrate how to handle cluster addons using Terraform.

### Create Cluster with Default Addons

By default, a PF9 cluster is initiated with default addons turned on. The following configuration sets up the cluster with all default addons and their standard configurations.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = []
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
}
```

### Create Cluster with Specific Addons

If you want to create a cluster with specific set of addons, these can be specified in the configuration. The configuration for the addons, if needed, can also be declared under the params attribute. Be aware that some addons may not function without the necessary configuration and it is advised to consult the addon documentation for the needed configuration. The document's final section contains a list of supported addon names along with their required and optional configurations.

The version of the addon to be installed can also be selected. If the version isn't specified, then the addon's default version will be installed. For instance, the following configuration installs the `0.67.0` version, superseding the default version of the `monitoring` addon.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = []
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
  addons = {
    "coredns" = {
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {}
    "metrics-server" = {
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }
    "monitoring" = {
      version = "0.67.0"
      params = {
        retentionTime = "6d"
      }
    }
  }
}
```

Be aware that if you create a cluster without providing the `addons` attribute, then you'll need to provide a full list of the default addons and their configurations (`params`) the next time you update the cluster addons. If at that point you specify just one new addon, all of the previous addons will be disabled, including the required ones.

### Enable the Addon

The code snippet below illustrates how to activate the `metallb` addon in an already existing cluster.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = []
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
  addons = {
    "coredns" = {
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {}
    "metrics-server" = {
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }
    "monitoring" = {
      version = "0.67.0"
      params = {
        retentionTime = "6d"
      }
    }
    # Enable the new addon
    "metallb" = {
        params = {
            MetallbIpRange = "192.168.5.0-192.168.6.0"
        }
    }
  }
}
```

### Disable the Addon

The code example below shows how to disable the `monitoring` addon in a cluster that's already set up.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = []
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
  addons = {
    "coredns" = {
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {}
    "metrics-server" = {
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }
    # Disable the addon
    # "monitoring" = {
    #   version = "0.67.0"
    #   params = {
    #     retentionTime = "6d"
    #   }
    # }
    "metallb" = {
        params = {
            MetallbIpRange = "192.168.5.0-192.168.6.0"
        }
    }
  }
}
```

### Upgrade the Addon

The configuration below demonstrates how to upgrade the monitoring addon to version `0.68.0` within an existing cluster. Similarly, the addon can also be downgraded using the same method.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = []
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
  addons = {
    "coredns" = {
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {}
    "metrics-server" = {
      # Upgrades the addon from current default version to 0.6.5
      version = "0.6.5"
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }
    "metallb" = {
        params = {
            MetallbIpRange = "192.168.5.0-192.168.6.0"
        }
    }
  }
}
```

## Default Addon Configurations

The following terraform configuration shows the default configurations for all the supported addons.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = []
  allow_workloads_on_master = true
  etcd_backup = {
    is_etcd_backup_enabled = true
  }
  addons = {
    "coredns" = {
      params = {
        dnsMemoryLimit = "170Mi" # required
        dnsDomain      = "cluster.local" # required
      }
    }
    "kubernetes-dashboard" = {}
    "metrics-server" = {
      params = {
        metricsCpuLimit    = "100m" # required
        metricsMemoryLimit = "300Mi" # required
      }
    }
    "monitoring" = {
      params = {
        retentionTime = "7d" # required
        storageClassName = "default" # optional
        pvcSize = "1Gi" # optional
      }
    }
    "metallb" = {
      params = {
        MetallbIpRange = "192.168.5.0-192.168.6.0" # required
      }
    }
    "kubevirt" = {}
    "luigi" = {}
    "metal3" = {
      params = {
        Metal3DhcpInterface = "ens3" # required
        Metal3DhcpRange = "192.168.52.230,192.168.52.250" # required
        Metal3DhcpGateway = "192.168.52.1" # required
        # optional params
        Metal3DnsServer = "8.8.8.8"
        Metal3KernelURL = "https://ironic-images.s3.us-west-1.amazonaws.com/metal3/ironic-agent.kernel"
        Metal3RamdiskURL = "https://ironic-images.s3.us-west-1.amazonaws.com/metal3/ironic-agent.initramfs"
        Metal3SshKey = "ssh-rsa AAA***PJL root@localhost.localdomain"
        StorageClassName = "db-sc"
      }
    }
  }
}
```