package pf9

import (
    "bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

type Pf9AWSCloudProvider struct {
	Name              string      `json:"name,omitempty"`
	Type              string      `json:"type,omitempty"`
	Key               string      `json:"key,omitempty"`
	Secret            string      `json:"secret,omitempty"`
}

const (
	Pf9AWSCloudProviderRetryTimeout = 2 * time.Minute
)


func resourcePf9AWSCloudProvider() *schema.Resource {
	return &schema.Resource{
		Create: resourcePf9AWSCloudProviderCreate,
		Read:   resourcePf9AWSCloudProviderRead,
		Update: resourcePf9AWSCloudProviderRead,
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
				Required: true,
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

	qbert_cloud_provider_api := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/cloudProviders"

	request := &Pf9AWSCloudProvider{
		Name:        d.Get("name").(string),
		Type:        d.Get("type").(string),
		Key:         d.Get("key").(string),
		Secret:      d.Get("secret").(string),
	}

	request_data, errJSON := json.Marshal(request)
	if errJSON != nil {
		return fmt.Errorf("Failed to parse request: %s", errJSON)
	}

	client := http.DefaultClient
	req, errReq := http.NewRequest("POST", qbert_cloud_provider_api, bytes.NewBuffer(request_data))
	if errReq != nil {
	    return fmt.Errorf("AWS Cloud Provider get failed: %s", errReq)
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	resp, errResp := client.Do(req)
	if errResp != nil {
		return fmt.Errorf("AWS Cloud Provider create failed: %s", errResp)
	}

    var resp_data struct {
		UUID string `json:"uuid"`
	}
	json.NewDecoder(resp.Body).Decode(&resp_data)

	d.SetId(resp_data.UUID)

	return resourcePf9AWSCloudProviderRead(d, meta)
}

func resourcePf9AWSCloudProviderRead(d *schema.ResourceData, meta interface{}) error {
    config := meta.(*Config)
	token, errToken := generateToken(config)
	if errToken != nil {
		return fmt.Errorf("Failed to generate token: %s", errToken)
	}
    cloudProviderUuid := d.Id()
	qbert_cloud_provider_get_api := "https://" + config.DuFQDN + "/qbert/v3/" + d.Get("project_uuid").(string) + "/cloudProviders/" + cloudProviderUuid

    client := http.DefaultClient
	req, errReq := http.NewRequest("GET", qbert_cloud_provider_get_api, nil)
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
	return nil
}