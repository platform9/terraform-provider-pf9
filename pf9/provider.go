package pf9

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

// Qbert API fields
type Qbert struct {
	WorkloadsOnMaster int                    `json:"allowWorkloadsOnMaster"`
	Ami               string                 `json:"ami,omitempty"`
	AppCatalogEnabled int                    `json:"appCatalogEnabled"`
	Azs               []string               `json:"azs,omitempty"`
        Zones             []string               `json:"zones,omitempty"`
	ContainersCIDR    string                 `json:"containersCidr,omitempty"`
	DomainID          string                 `json:"domainId"`
	ExternalDNSName   string                 `json:"externalDnsName"`
	HTTPProxy         string                 `json:"httpProxy,omitempty"`
	InternalElb       bool                   `json:"internalElb,omitempty"`
	IsPrivate         bool                   `json:"isPrivate,omitempty"`
	K8sAPIPort        string                 `json:"k8sApiPort,omitempty"`
	MasterFlavor      string                 `json:"masterFlavor,omitempty"`
        MasterSku         string                 `json:"masterSku,omitempty"`
	Name              string                 `json:"name,omitempty"`
	NetworkPlugin     string                 `json:"networkPlugin,omitempty"`
	CalicoIPIPMode    string                 `json:"calicoIpIpMode,omitempty"`
	CalicoNATOutgoing bool                   `json:"calicoNatOutgoing,omitempty"`
	NodePoolUUID      string                 `json:"nodePoolUuid,omitempty"`
	NumMasters        int                    `json:"numMasters,omitempty"`
	NumWorkers        int                    `json:"numWorkers,omitempty"`
	EnableCAS         bool                   `json:"enableCAS,omitempty"`
	NumSpotWorkers    int                    `json:"numSpotWorkers,omitempty"`
	NumMaxSpotWorkers int                    `json:"numMaxSpotWorkers,omitempty"`
	SpotPrice         float64                `json:"spotPrice,omitempty"`
	SpotWorkerFlavor  string                 `json:"spotWorkerFlavor,omitempty"`
	Masterless        int                    `json:"masterless,omitempty"`
	PrivateSubnets    []string               `json:"privateSubnets,omitempty"`
	Privileged        int                    `json:"privileged,omitempty"`
	Region            string                 `json:"region,omitempty"`
        Location          string                 `json:"location,omitempty"`
	RuntimeConfig     string                 `json:"runtimeConfig,omitempty"`
	ServiceFQDN       string                 `json:"serviceFqdn"`
	ServicesCIDR      string                 `json:"servicesCidr,omitempty"`
	SSHKey            string                 `json:"sshKey,omitempty"`
	Subnets           []string               `json:"subnets,omitempty"`
	Tags              map[string]interface{} `json:"tags,omitempty"`
	VPC               string                 `json:"vpc,omitempty"`
	WorkerFlavor      string                 `json:"workerFlavor,omitempty"`
        WorkerSku         string                 `json:"workerSku,omitempty"`
	MasterVIPIPv4     string                 `json:"masterVipIpv4,omitempty"`
	MasterVIPIface    string                 `json:"masterVipIface,omitempty"`
	EnableMetalLB     bool                   `json:"enableMetallb,omitempty"`
	MetalLBCIDR       string                 `json:"metallbCidr,omitempty"`
	ApiServerFlags    []string               `json:"apiServerFlags, omitempty"`
	SchedulerFlags    []string               `json:"schedulerFlags, omitempty"`
	CtrlrManagerFlags []string               `json:"controllerManagerFlags, omitempty"`
}

// Config stores the PF9 provider configuration options
type Config struct {
	DuFQDN           string
	DuUsername       string
	DuPassword       string
	IdentityEndpoint string
	DuTenantName     string
	TerraformVersion string
}

// Domain defines a keystone region
type Domain struct {
	ID string `json:"id"`
}

// User defines a keystone user
type User struct {
	Name     string `json:"name"`
	Domain   Domain `json:"domain"`
	Password string `json:"password"`
}

// Password defines the structure used to describe password based auth
type Password struct {
	User User `json:"user"`
}

// Identity defines the Identity keystone object
type Identity struct {
	Methods  []string `json:"methods"`
	Password Password `json:"password"`
}

// Project defines the structure used to describe the keystone project
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

// Provider returns a pf9 terraform resource provider
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
			"pf9_cluster":              resourcePF9Cluster(),
			"pf9_aws_cloud_provider":   resourcePf9AWSCloudProvider(),
			"pf9_azure_cloud_provider": resourcePf9AzureCloudProvider(),
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
	identityEndpoint := "https://" + d.Get("du_fqdn").(string) + "/keystone/v3/auth/tokens"
	config := Config{
		IdentityEndpoint: identityEndpoint,
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
							ID: "default",
						},
						Password: config.DuPassword,
					},
				},
			},
			Scope: Scope{
				Project: Project{
					Name: config.DuTenantName,
					Domain: Domain{
						ID: "default",
					},
				},
			},
		},
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	resp, errPost := http.Post(config.IdentityEndpoint, "application/json", bytes.NewBuffer(requestData))
	if errPost != nil {
		return "", errPost
	}

	token := resp.Header.Get("X-Subject-Token")
	return token, nil
}
