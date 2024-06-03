data "pf9_clusters" "example" {
  filters = [{
    name = "name"
    values = [ "mycluster01" ]
  }]
}

output "example" {
  value = data.pf9_clusters.example.cluster_ids[0]
}