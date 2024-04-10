data "pf9_host" "example" {
  id = "2c5f75a1-5fb3-4d18-b9df-b6313d483961"
}

output "ifaces" {
  value = data.pf9_host.example.interfaces
}

# Find the IP address of the ens3 interface
output "ens3_ip" {
  value = lookup(
    { for iface in data.pf9_host.example.interfaces : iface.name => iface.ip if iface.name == "ens3" },
    "ens3",
    ""
  )
}