package pf9

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

const (
	PF9ClusterRetryTimeout = 5 * time.Minute
)

type Qbert struct {
	WorkloadsOnMaster int           `json:"allowWorkloadsOnMaster,omitempty"`
	Ami               string        `json:"ami,omitempty"`
	AppCatalogEnabled int           `json:"appCatalogEnabled,omitempty"`
	Azs               []string      `json:"azs,omitempty"`
	ContainersCIDR    string        `json:"containersCidr,omitempty"`
	DomainID          string        `json:"domainId,omitempty"`
	ExternalDNSName   string        `json:"externalDnsName,omitempty"`
	HTTPProxy         string        `json:"httpProxy,omitempty"`
	InternalElb       bool          `json:"internalElb,omitempty"`
	IsPrivate         bool          `json:"isPrivate,omitempty"`
	K8sAPIPort        string        `json:"k8sApiPort,omitempty"`
	MasterFlavor      string        `json:"masterFlavor,omitempty"`
	Name              string        `json:"name,omitempty"`
	NetworkPlugin     string        `json:"networkPlugin,omitempty"`
	NodePoolUUID      string        `json:"nodePoolUuid,omitempty"`
	NumMasters        int           `json:"numMasters,omitempty"`
	NumWorkers        int           `json:"numWorkers,omitempty"`
	PrivateSubnets    []string      `json:"privateSubnets,omitempty"`
	Privileged        bool          `json:"privileged,omitempty"`
	Region            string        `json:"region,omitempty"`
	RuntimeConfig     string        `json:"runtimeConfig,omitempty"`
	ServiceFQDN       string        `json:"serviceFqdn,omitempty"`
	ServicesCIDR      string        `json:"servicesCidr,omitempty"`
	SSHKey            string        `json:"sshKey,omitempty"`
	Subnets           []string      `json:"subnets,omitempty"`
	Tags              []interface{} `json:"tags,omitempty"`
	UsePF9Domain      bool          `json:"usePf9Domain,omitempty"`
	VPC               string        `json:"vpc,omitempty"`
	WorkerFlavor      string        `json:"workerFlavor,omitempty"`
	MasterVIPIPv4     string        `json:"masterVipIpv4,omitempty"`
	MasterVIPIface    string        `json:"masterVipIface,omitempty"`
	EnableMetalLB     bool          `json:"enableMetallb,omitempty"`
	MetalLBCIDR       string        `json:"metallbCidr,omitempty"`
}

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
			"project_uuid": &schema.Schema{
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
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	qbert_cluster_api := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/clusters"

	request := &Qbert{
		WorkloadsOnMaster: d.Get("allowWorkloadsOnMaster").(int),
		Ami:               d.Get("ami").(string),
		AppCatalogEnabled: d.Get("appCatalogEnabled").(int),
		Azs:               d.Get("azs").([]string),
		ContainersCIDR:    d.Get("containersCidr").(string),
		DomainID:          d.Get("domainId").(string),
		ExternalDNSName:   d.Get("externalDnsName").(string),
		HTTPProxy:         d.Get("httpProxy").(string),
		InternalElb:       d.Get("internalElb").(bool),
		IsPrivate:         d.Get("isPrivate").(bool),
		K8sAPIPort:        d.Get("k8sApiPort").(string),
		MasterFlavor:      d.Get("masterFlavor").(string),
		Name:              d.Get("name").(string),
		NetworkPlugin:     d.Get("networkPlugin").(string),
		NodePoolUUID:      d.Get("nodePoolUuid").(string),
		NumMasters:        d.Get("numMasters").(int),
		NumWorkers:        d.Get("numWorkers").(int),
		PrivateSubnets:    d.Get("privateSubnets").([]string),
		Privileged:        d.Get("privileged").(bool),
		Region:            d.Get("region").(string),
		RuntimeConfig:     d.Get("runtimeConfig").(string),
		ServiceFQDN:       d.Get("serviceFqdn").(string),
		ServicesCIDR:      d.Get("servicesCidr").(string),
		SSHKey:            d.Get("sshKey").(string),
		Subnets:           d.Get("subnets").([]string),
		Tags:              d.Get("tags").([]interface{}),
		UsePF9Domain:      d.Get("usePf9Domain").(bool),
		VPC:               d.Get("vpc").(string),
		WorkerFlavor:      d.Get("workerFlavor").(string),
		MasterVIPIPv4:     d.Get("masterVipIpv4").(string),
		MasterVIPIface:    d.Get("masterVipIface").(string),
		EnableMetalLB:     d.Get("enableMetallb").(bool),
		MetalLBCIDR:       d.Get("metallbCidr").(string),
	}

	request_data, errJSON := json.Marshal(request)
	if errJSON != nil {
		return fmt.Errorf("Failed to parse request: %s", errJSON)
	}

	client := http.DefaultClient
	req, errReq := http.NewRequest("POST", qbert_cluster_api, bytes.NewBuffer(request_data))
	if errReq != nil {
		return fmt.Errorf("Failed to create cluster create request: %s", errReq)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	resp, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("Cluster create failed: %s", errResp)
	}

	var resp_data struct {
		UUID string `json:"uuid"`
	}
	json.NewDecoder(resp.Body).Decode(&resp_data)

	d.SetId(resp_data.UUID)

	return resourcePF9ClusterRead(d, meta)
}

func resourcePF9ClusterRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	qbert_cluster_api := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/clusters/" + d.Id()

	client := http.DefaultClient
	req, errReq := http.NewRequest("GET", qbert_cluster_api, nil)
	if errReq != nil {
		return fmt.Errorf("Failed to create cluster read request: %s", errReq)
	}
	req.Header.Add("X-Auth-Token", token)

	resp, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("Failed to get cluster details: %s", errResp)
	}

	var cluster Qbert
	json.NewDecoder(resp.Body).Decode(&cluster)

	d.Set("allowWorkloadsOnMaster", string(cluster.WorkloadsOnMaster))
	d.Set("ami", cluster.Ami)
	d.Set("appCatalogEnabled", string(cluster.AppCatalogEnabled))
	d.Set("azs", "["+strings.Join(cluster.Azs, ",")+"]")
	d.Set("containersCidr", cluster.ContainersCIDR)
	d.Set("domainId", cluster.DomainID)
	d.Set("externalDnsName", cluster.ExternalDNSName)
	d.Set("httpProxy", cluster.HTTPProxy)
	d.Set("internalElb", strconv.FormatBool(cluster.InternalElb))
	d.Set("isPrivate", strconv.FormatBool(cluster.IsPrivate))
	d.Set("k8sApiPort", cluster.K8sAPIPort)
	d.Set("masterFlavor", cluster.MasterFlavor)
	d.Set("name", cluster.Name)
	d.Set("networkPlugin", cluster.NetworkPlugin)
	d.Set("nodePoolUuid", cluster.NodePoolUUID)
	d.Set("numMasters", string(cluster.NumMasters))
	d.Set("numWorkers", string(cluster.NumWorkers))
	d.Set("privateSubets", "["+strings.Join(cluster.PrivateSubnets, ",")+"]")
	d.Set("privileged", strconv.FormatBool(cluster.Privileged))
	d.Set("region", cluster.Region)
	d.Set("runtimeConfig", cluster.RuntimeConfig)
	d.Set("serviceFqdn", cluster.ServiceFQDN)
	d.Set("servicesCidr", cluster.ServicesCIDR)
	d.Set("sshKey", cluster.SSHKey)
	d.Set("subnets", "["+strings.Join(cluster.Subnets, ",")+"]")
	d.Set("tags", fmt.Sprintf("%v", cluster.Tags...))
	d.Set("usePf9Domain", strconv.FormatBool(cluster.UsePF9Domain))
	d.Set("vpc", cluster.VPC)
	d.Set("workerFlavor", cluster.WorkerFlavor)
	d.Set("masterVipIpv4", cluster.MasterVIPIPv4)
	d.Set("masterVipIface", cluster.MasterVIPIface)
	d.Set("enableMetallb", strconv.FormatBool(cluster.EnableMetalLB))
	d.Set("metallbCidr", cluster.MetalLBCIDR)

	return nil
}

func resourcePF9ClusterUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourcePF9ClusterRead(d, meta)
}

func resourcePF9ClusterDelete(d *schema.ResourceData, meta interface{}) error {
	return nil
}
