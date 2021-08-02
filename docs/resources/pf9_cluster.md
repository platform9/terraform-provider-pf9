# pf9_cluster (Resource)

The pf9 resource allows you to create a cluster in a chosen cloud.

## Example Usage

```terraform
resource "pf9_cluster" "cluster_1" {
  project_uuid        = "<YOUR_P9_PROJECT_UUID>"
  cloud_provider_uuid = ""
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

## Schema

### Required

- **project_uuid** (String) PMK Project UUID. Learn how to find this value, [here](https://platform9.com/docs/kubernetes/introduction-to-platform9-uuid#tenants--project-uuid).
- **cloud_provider_uuid** (String) Cloud provider UUID. Provide this value if you want to use a cloud provider that has is already set up. In the P9 Console, navigate to the "Infrastructure" area and then the "Cloud Providers" tab. In the list of providers there will be a column labeled "Unique ID". Alternatly if you are creating a new cloud provider along with the cluster, set this to an empty string: "".
- **name** (String) Name of the cluster.
- **containers_cidr** (String) Subnet used for Pod IPs. Example: `10.20.0.0/16`
- **service_cidr** (String) Subnet used for Service IPs. Example: `10.21.0.0/16`
- **num_masters** (String) Number of masters. Recommended: 1, 3, or 5.
- **num_workers** (String) Number of workers. At least 1.

### Required if using AWS

- **ami** (String) AWS Image ID, example: "ubuntu"
- **region** (String) AWS region. Example: "us-east-1"
- **azs** (List) List of AWS availability zones. Example: ["us-east-1a", "us-east-1b"]
- **ssh_key** (String) Keyname for SSH access to nodes. For Azure it's the name of an [SSH key](https://docs.microsoft.com/en-us/azure/virtual-machines/ssh-keys-portal).
- **master_flavor** (String) Flavor of master nodes
- **worker_flavor** (String) Flavor of worker nodes

### Required if using Azure

- **location** (String) Azure region. Example: "eastus"
- **zones** (List) List of Azure availability zones. Example: ["1", "2"]
- **ssh_key** (String) Keyname for SSH access to nodes. Learn how to create and save a [key-pair](https://docs.microsoft.com/en-us/azure/virtual-machines/ssh-keys-portal) on aAzure.
- **master_sku** (String) Flavor of master nodes. Example: "Standard_A4_v2"
- **worker_sku** (String) Flavor of worker nodes. Example: "Standard_A4_v2"

### Optional

- **external_dns_name** (String) Provide DNS name. For AWS this will be an A record in the provided hosted zone, routing to the cluster's master instance(s) ELB. Example: `domain.com`
- **service_fqdn** (String) Provide FQDN for service endpoints. For AWS this will be an A record in the provided hosted zone, routing to the cluster's worker instance(s) ELB. Example: `my-services.domain.com`
- **privileged** (Int) Allow/disallow privileged containers. Available options: 1, [0]
- **http_proxy** (String) (optional) Specify the HTTP proxy for this cluster. Format: `<scheme>://<username>:<password>@<host>:<port>`
- **network_plugin** (String) Network plugin to use. Available options: [flannel], calico, canal(experimental)
- **calico_ip_ip_mode** (String) IP-IP mode if using the calico network plugin. Available options: Always, Never, CrossSubnet (default: Always)
- **calico_nat_outgoing** (Boolean) Enable outgoing NAT for calico nodes. Default value: true
- **tags** (Map) Tags to apply on nodes. Format: ["key1":"value1","key2":"value2"] (key-value pairs)
- **allow_workloads_on_master** (Int) Allow workloads on master nodes. Available options: [0], 1
- **app_catalog_enabled** (Int) Enable/Disable Platform9 [app catalog](https://platform9.com/docs/kubernetes/application-catalog). Available options: [0], 1
- **k8s_api_port** (String) Port on which the k8s API server listens on. Default value: 443
- **private_subnets** (List) List of private subnets to use. Example: `10.21.0.0/16`
- **enable_metal_lb** (Boolean) Enable/disable MetalLB. Available options: true, [false]
- **metallb_cidr** (String) MetalLB CIDR. Example: `10.21.0.0/16`
- **api_server_flags** (List) List of custom api server flags. Example: "--request-timeout=2m0s", "--kubelet-timeout=20s"
- **scheduler_flags** (List) List of scheduler flags. Example: "--kube-api-burst=120"
- **controller_manager_flags** (List) List of controller manager flags. Example: "--large-cluster-size-threshold=60"
- **internal_elb** (Boolean) Enable or disable elastic load balancer. Available options: true, [false]
- **enable_cas** (Boolean) Enable or disable cluster auto scaler. Available options: true, [false]
- **master_vip_ipv4** (String) Virtual IP for master nodes.

### Optional if using AWS

- **vpc** (String) Name of AWS VPC for nodes
- **domain_id** (String) AWS host zone id. This is found in the AWS hosted zones listing. Example: `123445sdfgfg`

### Danger zone

- **is_private** (Boolean) Private cluster (for advanced users only)
- **runtime_config** (String) Runtime config data.
- **subnets** (List) List of subnets to use (advanced)
- **master_vip_iface** (String) Interface to attach master VIP to.
