package pf9

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// AzureCloudProvider defines API fields for the Azure cloud provider
type AzureCloudProvider struct {
	ClientID       string `json:"clientId,omitempty"`
	ClientSecret   string `json:"clientSecret,omitempty"`
	Name           string `json:"name,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	TenantID       string `json:"tenantId,omitempty"`
	Type           string `json:"type,omitempty"`
}

// Timeouts for Azure cloud provider operations
const (
	Pf9AzureCloudProviderTimeout = 2 * time.Minute
)

func resourcePf9AzureCloudProvider() *schema.Resource {
	return &schema.Resource{
		Create: resourcePf9AzureCloudProviderCreate,
		Read:   resourcePf9AzureCloudProviderRead,
		Update: resourcePf9AzureCloudProviderUpdate,
		Delete: resourcePf9AzureCloudProviderDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(Pf9AzureCloudProviderTimeout),
			Update: schema.DefaultTimeout(Pf9AzureCloudProviderTimeout),
			Delete: schema.DefaultTimeout(Pf9AzureCloudProviderTimeout),
		},

		Schema: map[string]*schema.Schema{
			"project_uuid": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"client_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"client_secret": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"subscription_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"tenant_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"type": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourcePf9AzureCloudProviderCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	qbertCloudProviderAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/cloudProviders"

	request := &AzureCloudProvider{
		ClientID:       d.Get("client_id").(string),
		ClientSecret:   d.Get("client_secret").(string),
		Name:           d.Get("name").(string),
		SubscriptionID: d.Get("subscription_id").(string),
		TenantID:       d.Get("tenant_id").(string),
		Type:           d.Get("type").(string),
	}

	requestData, errJSON := json.Marshal(request)
	if errJSON != nil {
		return fmt.Errorf("Failed to parse request: %s", errJSON)
	}

	client := http.DefaultClient
	req, errReq := http.NewRequest("POST", qbertCloudProviderAPI, bytes.NewBuffer(requestData))
	if errReq != nil {
		return fmt.Errorf("AWS Cloud Provider create failed: %s", errReq)
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	resp, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("AWS Cloud Provider create failed: %s", errResp)
	}

	var respData struct {
		UUID string `json:"uuid"`
	}
	json.NewDecoder(resp.Body).Decode(&respData)

	d.SetId(respData.UUID)

	return resourcePf9AzureCloudProviderRead(d, meta)
}

func resourcePf9AzureCloudProviderRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}
	cloudProviderUUID := d.Id()
	qbertCloudProviderGetAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/cloudProviders/" + cloudProviderUUID

	client := http.DefaultClient
	req, errReq := http.NewRequest("GET", qbertCloudProviderGetAPI, nil)
	if errReq != nil {
		return fmt.Errorf("AWS Cloud Provider get failed: %s", errReq)
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	_, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("AWS Cloud Provider get failed: %s", errResp)
	}

	return nil
}

func resourcePf9AzureCloudProviderUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourcePf9AzureCloudProviderRead(d, meta)
}

func resourcePf9AzureCloudProviderDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}
	cloudProviderUUID := d.Id()
	qbertCloudProviderAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/cloudProviders/" + cloudProviderUUID

	client := http.DefaultClient
	req, errReq := http.NewRequest("DELETE", qbertCloudProviderAPI, nil)
	if errReq != nil {
		return fmt.Errorf("AWS Cloud Provider Delete failed: %s", errReq)
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	_, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("AWS Cloud Provider delete failed: %s", errResp)
	}

	d.SetId("")

	return nil
}
