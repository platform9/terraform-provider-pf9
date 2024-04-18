# Find Ubuntu hosts that are connected to PMK
data "pf9_hosts" "connected" {
  filters = [
    {
      name   = "responding"
      values = ["true"]
    },
    {
      name   = "os_info"
      regexes = ["Ubuntu.*"]
    }
  ]
}