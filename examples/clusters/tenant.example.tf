data "pf9_clusters" "example" {
    filters = [
        {
            name = "tenant"
            values = [ "service" ]
        }
    ]
}

# finds IDs of the clusters from the tenant service
output "example" {
  value = data.pf9_clusters.example.cluster_ids
}