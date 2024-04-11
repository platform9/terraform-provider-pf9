data "pf9_cluster" "example" {
  id = "d7727229-b2b4-4725-b0a0-473208f5093a"
}

output "example" {
  value = data.pf9_cluster.example.api_responding
}