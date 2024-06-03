terraform {
  required_providers {
    pf9 = {
      source  = "platform9/pf9"
      version = ">=0.3.0"
    }
  }
}

# Set the environemnt variable RXTSPOT_TOKEN to your Spot API token
provider "pf9" {}
