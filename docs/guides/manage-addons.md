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
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
}
```

### Create Cluster with Specific Addons

If you want to create a cluster with specific set of addons, these can be specified in the configuration. The configuration for the addons, if needed, can also be declared under the params attribute. Be aware that some addons may not function without the necessary configuration and it is advised to consult the [addon documentation for the needed configuration](https://platform9.com/docs/kubernetes/configuring-add-on-resource-requests-and-limits). The document's final section contains a list of supported addon names along with their required and optional configurations. For more information about pf9 addons, please refer to the official [documentation](https://platform9.com/docs/kubernetes/platform9-managed-add-ons-overview).

The version of the addon to be installed can also be selected. If the version isn't specified, then the addon's default version will be installed. For instance, the following configuration installs the `0.67.0` version, superseding the default version of the `monitoring` addon.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
  addons = {
    "coredns" = {
      enabled = true
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {
      enabled = true
    }
    "metrics-server" = {
      enabled = true
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }
    "monitoring" = {
      enabled = true
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

The code snippet below illustrates how to activate the `metallb` addon in an already existing cluster. You can use `enabled` flag to enable or disable the addon. The default value of `enabled` is `true`.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
  addons = {
    "coredns" = {
      enabled = true
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {
      enabled = true
    }
    "metrics-server" = {
      enabled = true
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }
    "monitoring" = {
      enabled = true
      version = "0.67.0"
      params = {
        retentionTime = "6d"
      }
    }
    # Enable the new addon
    "metallb" = {
      enabled = true
      params = {
          MetallbIpRange = "192.168.5.0-192.168.6.0"
      }
    }
  }
}
```

### Disable the Addon

The code example below shows how to disable the `monitoring` amd `metallb` addons from a cluster that's already set up.

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
  addons = {
    "coredns" = {
      enabled = true
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {
      enabled = true
    }
    "metrics-server" = {
      enabled = true
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }

    # Disable the monitoring addon
    "monitoring" = {
      enabled = false
      version = "0.67.0"
      params = {
        retentionTime = "6d"
      }
    }

    # Disable the metallb addon
    "metallb" = {
      enabled = false
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
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
  addons = {
    "coredns" = {
      enabled = true
      params = {
        dnsMemoryLimit = "170Mi"
        dnsDomain      = "cluster.local"
      }
    }
    "kubernetes-dashboard" = {}
    "metrics-server" = {
      enabled = true
      # Upgrades the addon from current default version to 0.6.5
      version = "0.6.5"
      params = {
        metricsMemoryLimit = "300Mi"
        metricsCpuLimit    = "100m"
      }
    }
    "metallb" = {
      enabled = true
      params = {
        MetallbIpRange = "192.168.5.0-192.168.6.0"
      }
    }
  }
}
```

### Add-on Health

Addon installation occurs asynchronously. After enabling an addon, you can confirm its installation status by checking the `phase` attribute. The `phase` attribute will indicate `Installed` once the addon is successfully installed. For instance, running `terraform state show pf9_cluster.example | grep phase` will return `phase = Installed` if the addon is installed. For more information about addon health, please refer to the official [documentation](https://platform9.com/docs/kubernetes/add-on-health).

## Manage `etcd_backup` Addon

Unlike other addons, the `etcd_backup` addon is configured separately from the `addons` attribute in the cluster configuration. To set up the `etcd_backup` addon, use the following configuration:

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
  etcd_backup = {
    daily = {
      # backup every day at 2:00 AM
      backup_time = "02:00"
      # delete backups older than 4 days
      max_backups_to_retain = 4
    }
    interval = {
      # backup every 50 minutes
      backup_interval = "50m"
      max_backups_to_retain = 5
    }
    storage_local_path = "/var/lib/etcd-backup"
    storage_type = "local"
  }
}
```

Set the `etcd_backup` attribute to `null` to disable it explicitly:

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
  # explicitly disabling the etcd_backup addons
  etcd_backup = null
}
```

## Default Addon Configurations

The following Terraform configuration displays the default settings for all supported addons. If you've disabled any default addons and wish to re-enable them, refer to this default configuration:

```terraform
resource "pf9_cluster" "example" {
  name                      = "tf-cluster-01"
  master_nodes              = [var.master_node]
  worker_nodes              = var.worker_nodes
  allow_workloads_on_master = true
  etcd_backup = {
    daily = {
      # backup every day at 2:00 AM
      backup_time = "02:00"
      # delete backups older than 4 days
      max_backups_to_retain = 4
    }
    interval = {
      # backup every 50 minutes
      backup_interval = "50m"
      max_backups_to_retain = 5
    }
    storage_local_path = "/var/lib/etcd-backup"
    storage_type = "local"
  }
  addons = {
    "coredns" = {
      enabled = true
      params = {
        dnsMemoryLimit = "170Mi" # required
        dnsDomain      = "cluster.local" # required
      }
    }
    "kubernetes-dashboard" = {
      enabled = true
    }
    "metrics-server" = {
      enabled = true
      params = {
        metricsCpuLimit    = "100m" # required
        metricsMemoryLimit = "300Mi" # required
      }
    }
    "monitoring" = {
      enabled = true
      params = {
        retentionTime = "7d" # required
        storageClassName = "default" # optional
        pvcSize = "1Gi" # optional
      }
    }
    "metallb" = {
      enabled = true
      params = {
        MetallbIpRange = "192.168.5.0-192.168.6.0" # required
      }
    }
    "kubevirt" = {
      enabled = true
    }
    "luigi" = {
      enabled = true
    }
    "metal3" = {
      enabled = true
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