package pf9

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	"github.com/gophercloud/utils/terraform/auth"
)

type Config struct {
	auth.Config
}

func Provider() terraform.ResourceProvider {
	provider := &schema.Provider{
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
				DefaultFunc: schema.EnvDefaultFunc("DU_PASSWORD", ""),
			},
			"du_tenant": {
				Type:        schema.TypeString,
				Required:    true,
				Description: descriptions["du_tenant"],
				DefaultFunc: schema.EnvDefaultFunc("DU_TENANT", ""),
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"pf9_cluster": resourcePF9Cluster(),
		},
	}

	provider.ConfigureFunc = func(d *schema.ResourceData) (interface{}, error) {
		terraformVersion := provider.TerraformVersion
		if terraformVersion == "" {
			terraformVersion = "0.11+compatible"
		}
		return configureProvider(d, terraformVersion)
	}

	return provider
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"du_fqdn":     "FQDN of the Platform9 Control Plane.",
		"du_username": "Platform9 Control Plane username.",
		"du_password": "Platform9 Control Plane password.",
	}
}

func configureProvider(d *schema.ResourceData, terraformVersion string) (interface{}, error) {
	identity_endpoint := "https://" + d.Get("du_fqdn").(string) + "/keystone/v3/auth/tokens"
	config := Config{
		auth.Config{
			IdentityEndpoint:  identity_endpoint,
			Username:          d.Get("du_username").(string),
			Password:          d.Get("du_password").(string),
			DomainName:        "default",
			ProjectDomainName: "default",
			TenantName:        d.Get("du_tenant").(string),
			TerraformVersion:  terraformVersion,
		},
	}

	if err := config.LoadAndValidate(); err != nil {
		return nil, err
	}

	return &config, nil
}
