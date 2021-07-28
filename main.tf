resource "pf9_aws_cloud_provider" "sample_aws_prov" {
    name       = "sample_aws_provider"
    type = "aws"
    key = "limepie"
    secret = "chamber"
    project_uuid = "5738c9a8269a42788c96d160047b5b1b"
}

resource "pf9_cluster" "cluster_1" {
    project_uuid = "5738c9a8269a42788c96d160047b5b1b"
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
    domain_id = "/hostedzone/DOMAINID"
    external_dns_name = "auto-generate"
    internal_elb = false
    is_private = "false"
    master_flavor = "t2.medium"
    name = "tf-sample-1"
    network_plugin = "flannel"
    cloud_provider_uuid = "5c703dc1-6037-44a6-b7f4-1d7fc89cbca6"
    privileged = 1
    region = "us-west-2"
    runtime_config = ""
    service_fqdn = "auto-generate"
    services_cidr = "10.21.0.0/16"
    ssh_key = "my-ssh-key"
    worker_flavor = "t2.medium"
    num_masters = 1
    num_workers = 3
    enable_cas = "false"
    masterless = 1
}
