data "pf9_nodes" "example" {
    # Filter is allowed on the following attributes: name, primary_ip, id, is_master
    filter = {
        name = "name"
        values = ["foobar"]
    }
}

# Outputs id of the node whose name is "foobar"
output "foobar" {
    value = data.pf9_nodes.example.nodes[0].id
}
