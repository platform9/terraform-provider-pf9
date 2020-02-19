package pf9

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

type Config struct {
	DuFQDN           string
	DuUsername       string
	DuPassword       string
	IdentityEndpoint string
	DuTenantName     string
	TerraformVersion string
}

type Domain struct {
	Id string `json:"id"`
}

type User struct {
	Name     string `json:"name"`
	Domain   Domain `json:"domain"`
	Password string `json:"password"`
}

type Password struct {
	User User `json:"user"`
}

type Identity struct {
	Methods  []string `json:"methods"`
	Password Password `json:"password"`
}

type Project struct {
	Name   string `json:"name"`
	Domain Domain `json:"domain"`
}

type Scope struct {
	Project Project `json:"project"`
}

type Auth struct {
	Identity Identity `json:"identity"`
	Scope    Scope    `json:"scope"`
}

type Request struct {
	Auth Auth `json:"auth"`
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
			"pf9_aws_cloud_provider": resourcePf9AWSCloudProvider(),
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
		IdentityEndpoint: identity_endpoint,
		DuUsername:       d.Get("du_username").(string),
		DuPassword:       d.Get("du_password").(string),
		DuTenantName:     d.Get("du_tenant").(string),
		TerraformVersion: terraformVersion,
		DuFQDN:           d.Get("du_fqdn").(string),
	}

	return &config, nil
}

func generateToken(config *Config) (string, error) {
	request := &Request{
		Auth: Auth{
			Identity: Identity{
				Methods: []string{"password"},
				Password: Password{
					User: User{
						Name: config.DuUsername,
						Domain: Domain{
							Id: "default",
						},
						Password: config.DuPassword,
					},
				},
			},
			Scope: Scope{
				Project: Project{
					Name: config.DuTenantName,
					Domain: Domain{
						Id: "default",
					},
				},
			},
		},
	}

	request_data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	resp, errPost := http.Post(config.IdentityEndpoint, "application/json", bytes.NewBuffer(request_data))
	if errPost != nil {
		return "", errPost
	}

	token := resp.Header.Get("X-Subject-Token")
	return token, nil
}
