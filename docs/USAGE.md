### Usage

Konform requires you to source access credentials to the Platform9 Saas Management Plane(DU) before running terraform. Please create and source an environment file with the following fields set:

```shell
export DU_USERNAME=<Platform9 DU Username>
export DU_PASSWORD=<Password>
export DU_FQDN=<FQDN>
export DU_TENANT=<Tenant Name>
```

Cluster configuration options are then added to the terraform script and run. A sample terraform script is in [main.tf](https://github.com/platform9/konform/blob/master/main.tf) for your reference.

## Terraform options
The following resources and their config options are currently supported.

### pf9_aws_cloud_provider
Create and manage an AWS cloud provider for PMK. Allowed config options:
```
name    (string) Name of the provider
type    (string) options: aws/azure
key     (string) AWS access key
secret  (string) AWS secret key
```

### pf9_azure_cloud_provider
Create and manage an AWS cloud provider for PMK. Allowed config options:
```
name            (string) Name of the provider
type            (string) options: aws/azure
project_uuid    (string) Azure project ID
client_id       (string) Azure Client ID
client_secret   (string) Azure Client Secret
subscription_id (string) Azure Subscription ID
tenant_id       (string) Azure Tenant ID
```

### pf9_cluster
Create and manage PMK clusters. Allowed config options:
```
project_uuid                (string)    PMK Project UUID
name                        (string)    Name of the cluster
allow_workloads_on_master   (int)       Allow workloads on master nodes options: 0/1
ami                         (string)    AWS clusters: AWS Image ID
app_catalog_enabled         (int)       Enable/Disable App Catalog. options: 0/1
azs                         (list)      List of AWS availability zones. Example: ["az1", "az2"]
zones                       (list)      List of Azure availability zones. Example: ["zone1, "zone2"]
containers_cidr             (string)    Subnet used for Pod IPs
service_cidr                (string)    Subnet used for Service IPs
domain_id                   (string)    AWS Domain ID
external_dns_name           (string)    "auto-generate", or provide DNS name
http_proxy                  (string)    (optional) Specify the HTTP proxy for this cluster. Format: <scheme>://<username>:<password>@<host>:<port>
internal_elb                (boolean)   Enable or disable elastic load balancer
is_private                  (boolean)   Private cluster (for advanced users only)
k8s_api_port                (string)    Port on which the k8s API server listens on
master_flavor               (string)    Flavor of master nodes (AWS)
worker_flavor               (string)    Flavor of worker nodes (AWS)
master_sku                  (string)    Flavor of master nodes (Azure)
worker_sku                  (string)    Flavor of worker nodes (Azure)
num_masters                 (string)    Number of masters. Recommended: 1, 3, or 5.
num_workers                 (string)    Number of workers.
enable_cas                  (boolean)   Enable or disable cluster auto scaler.
network_plugin              (string)    Network plugin to use: Available options: flannel, calico, canal(experimental)
calico_ip_ip_mode           (string)    IP-IP mode if using the calico network plugin. Available options: Always, Never, CrossSubnet (default: Always)
calico_nat_outgoing         (boolean)   Enable outgoing NAT for calico nodes (default: True)
node_pool_uuid              (string)    AWS node pool UUID
private_subnets             (list)      List of private subnets to use
privileged                  (int)       Allow/disallow privileged containers (0/1)    
region                      (string)    AWS region
location                    (string)    Azure region
runtime_config              (string)    Runtime config data
service_fqdn                (string)    "auto-generate" or provide FQDN for service endpoints
ssh_key                     (string)    Keyname for SSH access to nodes
subnets                     (list)      List of subnets to use (advanced)
tags                        (map)       Tags to apply on nodes (key-value pairs)
vpc                         (string)    Name of AWS VPC for nodes
master_vip_ipv4             (string)    Virtual IP for master nodes
master_vip_iface            (string)    Interface to attach master VIP to
enable_metal_lb             (boolean)   Enable/disable MetalLB
metallb_cidr                (string)    MetalLB CIDR
api_server_flags            (list)      List of custom api server flags. Example: ["--request-timeout=2m0s", "--kubelet-timeout=20s"]
scheduler_flags             (list)      List of scheduler flags. Example: ["--kube-api-burst=120"]
controller_manager_flags    (list)      List of controller manager flags. Example: ["--large-cluster-size-threshold=60"]
```
