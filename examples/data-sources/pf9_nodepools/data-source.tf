data "pf9_nodepools" "example" {
  # Finds all the nodepools with the name "defaultPool"
  filter = {
    name   = "name"
    values = ["defaultPool"]
  }
}

output "defaultnodepoolid" {
  value = data.pf9_nodepools.example.nodepools[0].id
}