terraform {
  required_providers {
    pf9 = {
      source  = "platform9/pf9"
      version = ">=0.3.0"
    }
  }
}

provider "pf9" {}
