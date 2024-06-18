data "pf9_nodes" "example" {
  # Filter is allowed on the following attributes:
  # id, name, api_responding, cluster_name, cluster_uuid, is_master, node_pool_name, node_pool_uuid, primary_ip, status
  filter = {
    name   = "name"
    values = ["foobar"]
  }
}

# Outputs id of the node whose name is "foobar"
output "foobar" {
  value = data.pf9_nodes.example.nodes[0].id
}

# Filter nodes attached to the cluster named "mycluster"
data "pf9_nodes" "example" {
  filter = {
    name   = "cluster_name"
    values = ["mycluster"]
  }
}