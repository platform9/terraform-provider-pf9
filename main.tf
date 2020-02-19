resource "pf9_aws_cloud_provider" "test_sup_aws_prov" {
    name       = "test_sup_aws_provider"
    type = "aws"
    key = "aedceceasca"
    secret = "vdfnajkvnd"
    project_uuid = "34a8c9a8269a42788c96d160047b5b1b"
}

resource "pf9_cluster" "cluster_1" {
    project_uuid = "34a8c9a8269a42788c96d160047b5b1b"
    
}