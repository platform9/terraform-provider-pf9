package pf9

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"du_fqdn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["du_fqdn"],
				DefaultFunc: schema.EnvDefaultFunc("DU_FQDN", ""),
			},
			"du_username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["du_username"],
				DefaultFunc: schema.EnvDefaultFunc("DU_USERNAME", ""),
			},
			"du_password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: descriptions["du_password"],
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"pf9_cluster": resourcePF9Cluster(),
		},
	}
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"du_fqdn":     "FQDN of the Platform9 Control Plane.",
		"du_username": "Platform9 Control Plane username.",
		"du_password": "Platform9 Control Plane password.",
	}
}
