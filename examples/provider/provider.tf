terraform {
  required_providers {
    pf9 = {
      source = "platform9/pf9"
    }
  }
}

# Set the variable value in *.tfvars file
# or using -var="account_url=..." CLI option
variable "account_url" {}
variable "username" {}
variable "password" {}

# Configure the Platform9 provider
provider "pf9" {
  account_url = var.account_url
  username    = var.username
  password    = var.password
}

# Create a cluster
resource "pf9_cluster" "example" {
  name         = "example-cluster"
  master_nodes = ["<node-id>"]
  # other attributes...
}