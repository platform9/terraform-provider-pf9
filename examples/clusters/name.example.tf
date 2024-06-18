data "pf9_clusters" "example" {
  filters = [
    {
      name    = "name"
      regexes = ["mycluster[0-9]+"]
    }
  ]
}

# finds IDs of the clusters that have the
# name matching to the regexes "mycluster[0-9]+"
output "example" {
  value = data.pf9_clusters.example.cluster_ids
}