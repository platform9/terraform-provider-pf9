package pf9

import (
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

const (
	PF9ClusterRetryTimeout = 5 * time.Minute
)

func resourcePF9Cluster() *schema.Resource {
	return &schema.Resource{
		Create: resourcePF9ClusterCreate,
		Read:   resourcePF9ClusterRead,
		Update: resourcePF9ClusterUpdate,
		Delete: resourcePF9ClusterDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(PF9ClusterRetryTimeout),
			Update: schema.DefaultTimeout(PF9ClusterRetryTimeout),
			Delete: schema.DefaultTimeout(PF9ClusterRetryTimeout),
		},

		Schema: map[string]*schema.Schema{
			"du_fqdn": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"du_username": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"du_password": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"allowWorkloadsOnMaster": &schema.Schema{
				Type:     schema.TypeInt,
				Required: false,
			},
			"ami": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"appCatalogEnabled": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"azs": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: false,
			},
			"containersCidr": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"domainId": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"externalDnsName": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"httpProxy": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"internalElb": &schema.Schema{
				Type:     schema.TypeBool,
				Required: false,
			},
			"isPrivate": &schema.Schema{
				Type:     schema.TypeBool,
				Required: false,
			},
			"k8sApiPort": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"masterFlavor": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"networkPlugin": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"nodePoolUuid": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"privateSubnets": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: false,
			},
			"privileged": &schema.Schema{
				Type:     schema.TypeBool,
				Required: false,
			},
			"region": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"runtimeConfig": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"serviceFqdn": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"servicesCidr": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"sshKey": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"subnets": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: false,
			},
			"tags": &schema.Schema{
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: false,
			},
			"usePf9Domain": &schema.Schema{
				Type:     schema.TypeBool,
				Required: false,
			},
			"vpc": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"workerFlavor": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"masterVipIpv4": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"masterVipIface": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
			"enableMetallb": &schema.Schema{
				Type:     schema.TypeBool,
				Required: false,
			},
			"metallbCidr": &schema.Schema{
				Type:     schema.TypeString,
				Required: false,
			},
		},
	}
}

func resourcePF9ClusterCreate(d *schema.ResourceData, meta interface{}) error {
	return resourcePF9ClusterRead(d, meta)
}

func resourcePF9ClusterRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourcePF9ClusterUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourcePF9ClusterRead(d, meta)
}

func resourcePF9ClusterDelete(d *schema.ResourceData, meta interface{}) error {
	return nil
}
