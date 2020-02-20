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
	WorkloadsOnMaster string        `json:"allowWorkloadsOnMaster,omitempty"`
	Ami               string        `json:"ami,omitempty"`
	AppCatalogEnabled string        `json:"appCatalogEnabled,omitempty"`
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
	EnableCAS         string        `json:"enableCAS,omitempty"`
	Masterless        string        `json:"masterless,omitempty"`
	PrivateSubnets    []string      `json:"privateSubnets,omitempty"`
	Privileged        bool          `json:"privileged,omitempty"`
	Region            string        `json:"region,omitempty"`
	RuntimeConfig     string        `json:"runtimeConfig,omitempty"`
	ServiceFQDN       string        `json:"serviceFqdn,omitempty"`
	ServicesCIDR      string        `json:"servicesCidr,omitempty"`
	SSHKey            string        `json:"sshKey,omitempty"`
	Subnets           []string      `json:"subnets,omitempty"`
	Tags              []string      `json:"tags,omitempty"`
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
			"allow_workloads_on_master": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"ami": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"app_catalog_enabled": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"azs": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"containers_cidr": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"domain_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"external_dns_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"http_proxy": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"internal_elb": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"is_private": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"k8s_api_port": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"master_flavor": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"num_masters": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"num_workers": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"enable_cas":  &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"masterless":  &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"network_plugin": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"node_pool_uuid": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"private_subnets": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"privileged": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"region": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"runtime_config": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"service_fqdn": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"services_cidr": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"ssh_key": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"subnets": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"tags": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"use_pf9_domain": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"vpc": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"worker_flavor": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"master_vip_ipv4": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"master_vip_iface": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"enable_metal_lb": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"metallb_cidr": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func convertIntfListToString(data []interface{}) []string {
	conv := make([]string, len(data))
	for i, data := range data {
		conv[i] = data.(string)
	}
	return conv
}

func resourcePF9ClusterCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	qbert_cluster_api := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/clusters"

    azs := convertIntfListToString(d.Get("azs").([]interface{}))
    PrivateSubnets := convertIntfListToString(d.Get("private_subnets").([]interface{}))
    Subnets := convertIntfListToString(d.Get("subnets").([]interface{}))
    Tags := convertIntfListToString(d.Get("tags").([]interface{}))

	request := &Qbert{
		WorkloadsOnMaster: d.Get("allow_workloads_on_master").(string),
		Ami:               d.Get("ami").(string),
		AppCatalogEnabled: d.Get("app_catalog_enabled").(string),
		Azs:               azs,
		ContainersCIDR:    d.Get("containers_cidr").(string),
		DomainID:          d.Get("domain_id").(string),
		ExternalDNSName:   d.Get("external_dns_name").(string),
		HTTPProxy:         d.Get("http_proxy").(string),
		InternalElb:       d.Get("internal_elb").(bool),
		IsPrivate:         d.Get("is_private").(bool),
		K8sAPIPort:        d.Get("k8s_api_port").(string),
		MasterFlavor:      d.Get("master_flavor").(string),
		Name:              d.Get("name").(string),
		NetworkPlugin:     d.Get("network_plugin").(string),
		NodePoolUUID:      d.Get("node_pool_uuid").(string),
		NumMasters:        d.Get("num_masters").(int),
		NumWorkers:        d.Get("num_workers").(int),
		EnableCAS:         d.Get("enable_cas").(string),
		Masterless:        d.Get("masterless").(string),
		PrivateSubnets:    PrivateSubnets,
		Privileged:        d.Get("privileged").(bool),
		Region:            d.Get("region").(string),
		RuntimeConfig:     d.Get("runtime_config").(string),
		ServiceFQDN:       d.Get("service_fqdn").(string),
		ServicesCIDR:      d.Get("services_cidr").(string),
		SSHKey:            d.Get("ssh_key").(string),
		Subnets:           Subnets,
		Tags:              Tags,
		UsePF9Domain:      d.Get("use_pf9_domain").(bool),
		VPC:               d.Get("vpc").(string),
		WorkerFlavor:      d.Get("worker_flavor").(string),
		MasterVIPIPv4:     d.Get("master_vip_ipv4").(string),
		MasterVIPIface:    d.Get("master_vip_iface").(string),
		EnableMetalLB:     d.Get("enable_metal_lb").(bool),
		MetalLBCIDR:       d.Get("metallb_cidr").(string),
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

	d.Set("allow_workloads_on_master", string(cluster.WorkloadsOnMaster))
	d.Set("ami", cluster.Ami)
	d.Set("app_catalog_enabled", string(cluster.AppCatalogEnabled))
	d.Set("azs", "["+strings.Join(cluster.Azs, ",")+"]")
	d.Set("containers_cidr", cluster.ContainersCIDR)
	d.Set("domain_id", cluster.DomainID)
	d.Set("external_dns_name", cluster.ExternalDNSName)
	d.Set("http_proxy", cluster.HTTPProxy)
	d.Set("internal_elb", strconv.FormatBool(cluster.InternalElb))
	d.Set("is_private", strconv.FormatBool(cluster.IsPrivate))
	d.Set("k8s_api_port", cluster.K8sAPIPort)
	d.Set("master_flavor", cluster.MasterFlavor)
	d.Set("name", cluster.Name)
	d.Set("network_plugin", cluster.NetworkPlugin)
	d.Set("node_pool_uuid", cluster.NodePoolUUID)
	d.Set("num_masters", string(cluster.NumMasters))
	d.Set("num_workers", string(cluster.NumWorkers))
	d.Set("enable_cas", string(cluster.EnableCAS))
	d.Set("masterless", string(cluster.Masterless))
	d.Set("private_subnets", "["+strings.Join(cluster.PrivateSubnets, ",")+"]")
	d.Set("privileged", strconv.FormatBool(cluster.Privileged))
	d.Set("region", cluster.Region)
	d.Set("runtime_config", cluster.RuntimeConfig)
	d.Set("service_fqdn", cluster.ServiceFQDN)
	d.Set("services_cidr", cluster.ServicesCIDR)
	d.Set("ssh_key", cluster.SSHKey)
	d.Set("subnets", "["+strings.Join(cluster.Subnets, ",")+"]")
	d.Set("tags", "["+strings.Join(cluster.Tags, ",")+"]")
	d.Set("use_pf9_domain", strconv.FormatBool(cluster.UsePF9Domain))
	d.Set("vpc", cluster.VPC)
	d.Set("worker_flavor", cluster.WorkerFlavor)
	d.Set("master_vip_ipv4", cluster.MasterVIPIPv4)
	d.Set("master_vip_iface", cluster.MasterVIPIface)
	d.Set("enable_metal_lb", strconv.FormatBool(cluster.EnableMetalLB))
	d.Set("metallb_cidr", cluster.MetalLBCIDR)

	return nil
}

func resourcePF9ClusterUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourcePF9ClusterRead(d, meta)
}

func resourcePF9ClusterDelete(d *schema.ResourceData, meta interface{}) error {
	return nil
}
