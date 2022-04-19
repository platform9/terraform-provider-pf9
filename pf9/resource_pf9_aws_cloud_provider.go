package pf9

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// AWSCloudProvider defines API fields for the AWS cloud provider
type AWSCloudProvider struct {
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"`
	Key    string `json:"key,omitempty"`
	Secret string `json:"secret,omitempty"`
}

// Timeouts for AWS cloud provider operations
const (
	Pf9AWSCloudProviderRetryTimeout = 2 * time.Minute
)

func resourcePf9AWSCloudProvider() *schema.Resource {
	return &schema.Resource{
		Create: resourcePf9AWSCloudProviderCreate,
		Read:   resourcePf9AWSCloudProviderRead,
		Update: resourcePf9AWSCloudProviderUpdate,
		Delete: resourcePf9AWSCloudProviderDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(Pf9AWSCloudProviderRetryTimeout),
			Update: schema.DefaultTimeout(Pf9AWSCloudProviderRetryTimeout),
			Delete: schema.DefaultTimeout(Pf9AWSCloudProviderRetryTimeout),
		},

		Schema: map[string]*schema.Schema{
			"project_uuid": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"type": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"key": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"secret": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourcePf9AWSCloudProviderCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	qbertCloudProviderAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/cloudProviders"

	request := &AWSCloudProvider{
		Name:   d.Get("name").(string),
		Type:   d.Get("type").(string),
		Key:    d.Get("key").(string),
		Secret: d.Get("secret").(string),
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

	return resourcePf9AWSCloudProviderRead(d, meta)
}

func resourcePf9AWSCloudProviderRead(d *schema.ResourceData, meta interface{}) error {
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

func resourcePf9AWSCloudProviderDelete(d *schema.ResourceData, meta interface{}) error {
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

func resourcePf9AWSCloudProviderUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}

	cloudProviderUUID := d.Id()
	qbertCloudProviderAPI := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/cloudProviders/" + cloudProviderUUID

	if d.HasChange("type") {
		return fmt.Errorf("Cannot change type of cloud provider")
	}

	request := &AWSCloudProvider{
		Name:   d.Get("name").(string),
		Key:    d.Get("key").(string),
		Secret: d.Get("secret").(string),
	}

	requestData, errJSON := json.Marshal(request)
	if errJSON != nil {
		return fmt.Errorf("Failed to parse request: %s", errJSON)
	}

	client := http.DefaultClient
	req, errReq := http.NewRequest("PUT", qbertCloudProviderAPI, bytes.NewBuffer(requestData))
	if errReq != nil {
		return fmt.Errorf("AWS Cloud Provider update failed: %s", errReq)
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	_, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("AWS Cloud Provider update failed: %s", errResp)
	}

	return resourcePf9AWSCloudProviderRead(d, meta)
}
