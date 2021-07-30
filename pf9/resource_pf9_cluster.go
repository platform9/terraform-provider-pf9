package pf9

import (
	"io/ioutil"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// Timeout for qbert cluster operations
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
			"project_uuid": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"cloud_provider_uuid": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"allow_workloads_on_master": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"ami": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"app_catalog_enabled": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"azs": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"zones": &schema.Schema{
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
			"master_sku": &schema.Schema{
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
			"enable_cas": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"num_spot_workers": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"num_max_spot_workers": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"spot_price": &schema.Schema{
				Type:     schema.TypeFloat,
				Optional: true,
			},
			"spot_worker_flavor": &schema.Schema{
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
			"calico_ip_ip_mode": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Always",
			},
			"calico_nat_outgoing": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"node_pool_uuid": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"private_subnets": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"privileged": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"region": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"location": &schema.Schema{
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
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
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
			"worker_sku": &schema.Schema{
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
			"api_server_flags": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
				        Type: schema.TypeString,
				},      
				Optional: true,
			},      
			"scheduler_flags": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
				        Type: schema.TypeString,
				},      
				Optional: true,
			},      
			"controller_manager_flags": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
				        Type: schema.TypeString,
				},      
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
	qbertNodePoolGetAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/nodePools"

	client := http.DefaultClient

	req, errReq := http.NewRequest("GET", qbertNodePoolGetAPI, nil)
	if errReq != nil {
		return fmt.Errorf("Node Pool get failed: %s", errReq)
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	resp, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("Node Pool get failed: %s", errResp)
	}

	defer resp.Body.Close()

	respData, errRead := ioutil.ReadAll(resp.Body)
	if errRead != nil {
		return fmt.Errorf("Error reading response data: %s", errRead)
	}

	type nodePoolStruct struct {
		NodePoolUuid string `json:"uuid"`
		CloudProviderUuid string `json:"cloudProviderUuid"`
	}

	var x []*nodePoolStruct

	errParse := json.Unmarshal(respData, &x)
	if errParse != nil {
		return fmt.Errorf("Error parsing response data %s", errParse)
	}

	for _, v := range x {
		if v.CloudProviderUuid == d.Get("cloud_provider_uuid").(string) {
			d.Set("node_pool_uuid", v.NodePoolUuid)
		}
	}

	qbertClusterAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/clusters"

	azs := convertIntfListToString(d.Get("azs").([]interface{}))
	zones := convertIntfListToString(d.Get("zones").([]interface{}))
	PrivateSubnets := convertIntfListToString(d.Get("private_subnets").([]interface{}))
	Subnets := convertIntfListToString(d.Get("subnets").([]interface{}))
	ApiServerFlags := convertIntfListToString(d.Get("api_server_flags").([]interface{}))
	SchedulerFlags := convertIntfListToString(d.Get("scheduler_flags").([]interface{}))
	CtrlrManagerFlags := convertIntfListToString(d.Get("controller_manager_flags").([]interface{}))

	request := &Qbert{
		WorkloadsOnMaster: d.Get("allow_workloads_on_master").(int),
		Ami:               d.Get("ami").(string),
		AppCatalogEnabled: d.Get("app_catalog_enabled").(int),
		Azs:               azs,
		Zones:             zones,
		ContainersCIDR:    d.Get("containers_cidr").(string),
		DomainID:          d.Get("domain_id").(string),
		ExternalDNSName:   d.Get("external_dns_name").(string),
		HTTPProxy:         d.Get("http_proxy").(string),
		InternalElb:       d.Get("internal_elb").(bool),
		IsPrivate:         d.Get("is_private").(bool),
		K8sAPIPort:        d.Get("k8s_api_port").(string),
		MasterFlavor:      d.Get("master_flavor").(string),
		MasterSku:         d.Get("master_sku").(string),
		Name:              d.Get("name").(string),
		NetworkPlugin:     d.Get("network_plugin").(string),
		CalicoIPIPMode:    d.Get("calico_ip_ip_mode").(string),
		CalicoNATOutgoing: d.Get("calico_nat_outgoing").(bool),
		NodePoolUUID:      d.Get("node_pool_uuid").(string),
		NumMasters:        d.Get("num_masters").(int),
		NumWorkers:        d.Get("num_workers").(int),
		EnableCAS:         d.Get("enable_cas").(bool),
		NumSpotWorkers:    d.Get("num_spot_workers").(int),
		NumMaxSpotWorkers: d.Get("num_max_spot_workers").(int),
		SpotPrice:         d.Get("spot_price").(float64),
		SpotWorkerFlavor:  d.Get("spot_worker_flavor").(string),
		PrivateSubnets:    PrivateSubnets,
		Privileged:        d.Get("privileged").(int),
		Region:            d.Get("region").(string),
		Location:          d.Get("location").(string),
		RuntimeConfig:     d.Get("runtime_config").(string),
		ServiceFQDN:       d.Get("service_fqdn").(string),
		ServicesCIDR:      d.Get("services_cidr").(string),
		SSHKey:            d.Get("ssh_key").(string),
		Subnets:           Subnets,
		Tags:              d.Get("tags").(map[string]interface{}),
		VPC:               d.Get("vpc").(string),
		WorkerFlavor:      d.Get("worker_flavor").(string),
		WorkerSku:         d.Get("worker_sku").(string),
		MasterVIPIPv4:     d.Get("master_vip_ipv4").(string),
		MasterVIPIface:    d.Get("master_vip_iface").(string),
		EnableMetalLB:     d.Get("enable_metal_lb").(bool),
		MetalLBCIDR:       d.Get("metallb_cidr").(string),
		ApiServerFlags:    ApiServerFlags,
		SchedulerFlags:    SchedulerFlags,
		CtrlrManagerFlags: CtrlrManagerFlags,
	}

	requestData, errJSON := json.Marshal(request)
	if errJSON != nil {
		return fmt.Errorf("Failed to parse request: %s", errJSON)
	}

	createReq, errReq := http.NewRequest("POST", qbertClusterAPI, bytes.NewBuffer(requestData))
	if errReq != nil {
		return fmt.Errorf("Failed to create cluster create request: %s", errReq)
	}
	createReq.Header.Add("X-Auth-Token", token)
	createReq.Header.Add("Content-Type", "application/json")

	createResp, errResp := client.Do(createReq)
	if errResp != nil {
		return fmt.Errorf("Cluster create failed: %s", errResp)
	}

	var createRespData struct {
		UUID string `json:"uuid"`
	}
	json.NewDecoder(createResp.Body).Decode(&createRespData)

	d.SetId(createRespData.UUID)

	return resourcePF9ClusterRead(d, meta)
}

func resourcePF9ClusterRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	qbertClusterAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/clusters/" + d.Id()

	client := http.DefaultClient
	req, errReq := http.NewRequest("GET", qbertClusterAPI, nil)
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

	d.Set("allow_workloads_on_master", fmt.Sprint(cluster.WorkloadsOnMaster))
	d.Set("ami", cluster.Ami)
	d.Set("app_catalog_enabled", fmt.Sprint(cluster.AppCatalogEnabled))
	d.Set("azs", "["+strings.Join(cluster.Azs, ",")+"]")
	d.Set("zones", "["+strings.Join(cluster.Zones, ",")+"]")
	d.Set("containers_cidr", cluster.ContainersCIDR)
	d.Set("domain_id", cluster.DomainID)
	d.Set("external_dns_name", cluster.ExternalDNSName)
	d.Set("http_proxy", cluster.HTTPProxy)
	d.Set("internal_elb", strconv.FormatBool(cluster.InternalElb))
	d.Set("is_private", strconv.FormatBool(cluster.IsPrivate))
	d.Set("k8s_api_port", cluster.K8sAPIPort)
	d.Set("master_flavor", cluster.MasterFlavor)
	d.Set("master_sku", cluster.MasterSku)
	d.Set("name", cluster.Name)
	d.Set("network_plugin", cluster.NetworkPlugin)
	d.Set("calico_ip_ip_mode", cluster.CalicoIPIPMode)
	d.Set("calico_nat_outgoing", strconv.FormatBool(cluster.CalicoNATOutgoing))
	d.Set("node_pool_uuid", cluster.NodePoolUUID)
	d.Set("num_masters", fmt.Sprint(cluster.NumMasters))
	d.Set("num_workers", fmt.Sprint(cluster.NumWorkers))
	d.Set("enable_cas", strconv.FormatBool(cluster.EnableCAS))
	d.Set("num_spot_workers", fmt.Sprint(cluster.NumSpotWorkers))
	d.Set("num_max_spot_workers", fmt.Sprint(cluster.NumMaxSpotWorkers))
	d.Set("spot_price", fmt.Sprintf("%f", cluster.SpotPrice))
	d.Set("spot_worker_flavor", cluster.SpotWorkerFlavor)
	d.Set("private_subnets", "["+strings.Join(cluster.PrivateSubnets, ",")+"]")
	d.Set("privileged", fmt.Sprint(cluster.Privileged))
	d.Set("region", cluster.Region)
	d.Set("location", cluster.Location)
	d.Set("runtime_config", cluster.RuntimeConfig)
	d.Set("service_fqdn", cluster.ServiceFQDN)
	d.Set("services_cidr", cluster.ServicesCIDR)
	d.Set("ssh_key", cluster.SSHKey)
	d.Set("subnets", "["+strings.Join(cluster.Subnets, ",")+"]")
	d.Set("tags", fmt.Sprintf("%v", cluster.Tags))
	d.Set("vpc", cluster.VPC)
	d.Set("worker_flavor", cluster.WorkerFlavor)
	d.Set("worker_sku", cluster.WorkerSku)
	d.Set("master_vip_ipv4", cluster.MasterVIPIPv4)
	d.Set("master_vip_iface", cluster.MasterVIPIface)
	d.Set("enable_metal_lb", strconv.FormatBool(cluster.EnableMetalLB))
	d.Set("metallb_cidr", cluster.MetalLBCIDR)
	d.Set("api_server_flags", "["+strings.Join(cluster.ApiServerFlags, ",")+"]")
	d.Set("scheduler_flags", "["+strings.Join(cluster.SchedulerFlags, ",")+"]")
	d.Set("controller_manager_flags", "["+strings.Join(cluster.CtrlrManagerFlags, ",")+"]")

	return nil
}

func resourcePF9ClusterUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourcePF9ClusterRead(d, meta)
}

func resourcePF9ClusterDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	qbertClusterAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/clusters/" + d.Id()

	client := http.DefaultClient
	req, errReq := http.NewRequest("DELETE", qbertClusterAPI, nil)
	if errReq != nil {
		return fmt.Errorf("Failed to create cluster delete request: %s", errReq)
	}
	req.Header.Add("X-Auth-Token", token)

	_, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("Failed to delete cluster: %s", errResp)
	}

	d.SetId("")

	return nil
}
