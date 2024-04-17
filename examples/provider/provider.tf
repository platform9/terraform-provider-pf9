terraform {
  required_providers {
    pf9 = {
      source  = "platform9/pf9"
    }
  }
}

provider "pf9" {
  account_url = "<PF9_ACCOUNT_URL>"
  username    = "<PF9_USERNAME>"
  password    = "<PF9_PASSWORD>"
}
