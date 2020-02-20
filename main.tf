resource "pf9_aws_cloud_provider" "test_sup_aws_prov" {
    name       = "test_sup_aws_provider"
    type = "aws"
    key = "aedceceasca"
    secret = "vdfnajkvnd"
    project_uuid = "34a8c9a8269a42788c96d160047b5b1b"
}

resource "pf9_cluster" "cluster_1" {
    project_uuid = "34a8c9a8269a42788c96d160047b5b1b"
    allow_workloads_on_master = 0
    ami = "ubuntu"
    app_catalog_enabled = 0
    azs = [
        "us-west-2a",
        "us-west-2b",
        "us-west-2c",
        "us-west-2d"
    ]
    containers_cidr = "10.20.0.0/16"
    domain_id = "/hostedzone/Z2LZB5ZNQY6JC2"
    external_dns_name = "auto-generate"
    internal_elb = false
    is_private = "false"
    master_flavor = "t2.medium"
    name = "supreeth-tf-test-1"
    network_plugin = "flannel"
    node_pool_uuid = "9d103dc1-6037-44a6-b7f4-1d7fc89cbca6"
    privileged = 1
    region = "us-west-2"
    runtime_config = ""
    service_fqdn = "auto-generate"
    services_cidr = "10.21.0.0/16"
    ssh_key = "supreeth"
    use_pf9_domain = "true"
    worker_flavor = "t2.medium"
    num_masters = 1
    num_workers = 3
    enable_cas = "false"
    masterless = 1
}
