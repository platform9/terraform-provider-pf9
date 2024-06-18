data "pf9_clusters" "example" {
  filters = [
    {
      name   = "tags:env"
      values = ["production", "staging"]
    },
    {
      name   = "tags:app"
      values = ["nginx"]
    }
  ]
}

data "pf9_cluster" "example" {
  id = data.pf9_clusters.example.cluster_ids[0]
}

# finds name of the cluster that has the
# tag "env=production" and "app=nginx" OR
# "env=staging" and "app=nginx"
output "example" {
  value = data.pf9_cluster.example.name
}