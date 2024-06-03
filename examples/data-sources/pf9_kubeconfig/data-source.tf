data "pf9_clusters" "example" {
  filters = [{
    name = "name"
    values = [ "mycluster01" ]
  }]
}

data "pf9_kubeconfig" "example" {
  id = data.pf9_clusters.example.cluster_ids[0]
  authentication_method = "token"
}

resource "local_file" "kubeconfig" {
  count      = 1
  content    = data.pf9_kubeconfig.example.raw
  filename   = "${path.root}/kubeconfig"
}