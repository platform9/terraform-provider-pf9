package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v2"

	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_kubeconfig"
)

var _ datasource.DataSource = (*kubeconfigDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*kubeconfigDataSource)(nil)

func NewKubeconfigDataSource() datasource.DataSource {
	return &kubeconfigDataSource{}
}

type kubeconfigDataSource struct {
	client *pmk.HTTPClient
}

func (d *kubeconfigDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

func (d *kubeconfigDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_kubeconfig.KubeconfigDataSourceSchema(ctx)
}

func (d *kubeconfigDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *kubeconfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_kubeconfig.KubeconfigModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	clusterID := data.Id.ValueString()
	authenticationMethod := data.AuthenticationMethod.ValueString()

	// Read API call logic
	authInfo, err := d.client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}

	token := authInfo.Token
	projectID := authInfo.ProjectID
	opts := qbert.KubeconfigOptions{}
	if authenticationMethod == "certificate" {
		opts.ForceCertAuth = true
	}
	kubeconfigBlob, err := d.client.Qbert().GetClusterKubeconfig(projectID, clusterID, token, opts)
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster kubeconfig", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster kubeconfig", err.Error())
		return
	}
	kubeconfigStr := string(kubeconfigBlob)
	if authenticationMethod == "password" {
		tflog.Debug(ctx, "Replacing token with base64 encoded username and password")
		basicAuthToken := getBasicAuthToken()
		kubeconfigStr = strings.Replace(kubeconfigStr, token, basicAuthToken, 1)
		token = basicAuthToken
	}
	data.Raw = types.StringValue(kubeconfigStr)
	kubeConfig := KubeConfig{}
	err = yaml.Unmarshal(kubeconfigBlob, &kubeConfig)
	if err != nil {
		tflog.Error(ctx, "Failed to unmarshal kubeconfig", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to unmarshal kubeconfig", err.Error())
		return
	}

	usersMap := map[string]User{}
	for _, user := range kubeConfig.Users {
		usersMap[user.Name] = user
	}
	clustersMap := map[string]Cluster{}
	for _, cluster := range kubeConfig.Clusters {
		clustersMap[cluster.Name] = cluster
	}

	kcSlice := []datasource_kubeconfig.KubeconfigsValue{}
	for _, kubectx := range kubeConfig.Contexts {
		kc := datasource_kubeconfig.KubeconfigsValue{
			Cluster:  types.StringValue(kubectx.Context.Cluster),
			Username: types.StringValue(kubectx.Context.User),
			Name:     types.StringValue(kubectx.Name),
		}
		if user, ok := usersMap[kubectx.Context.User]; ok {
			if user.User.Token != "" {
				kc.Token = types.StringValue(user.User.Token)
			} else {
				kc.Token = types.StringNull()
			}
			if user.User.ClientCertificateData != "" {
				kc.ClientCertificate = types.StringValue(user.User.ClientCertificateData)
			} else {
				kc.ClientCertificate = types.StringNull()
			}
			if user.User.ClientKeyData != "" {
				kc.ClientKey = types.StringValue(user.User.ClientKeyData)
			} else {
				kc.ClientKey = types.StringNull()
			}
		}
		if cluster, ok := clustersMap[kubectx.Context.Cluster]; ok {
			kc.Host = types.StringValue(cluster.Cluster.Server)
			if cluster.Cluster.CertificateAuthorityData != "" {
				kc.ClusterCaCertificate = types.StringValue(cluster.Cluster.CertificateAuthorityData)
			} else {
				kc.ClusterCaCertificate = types.StringNull()
			}
		}
		kcObjValue, diags := kc.ToObjectValue(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		kcObjValuable, diags := datasource_kubeconfig.KubeconfigsType{}.ValueFromObject(ctx, kcObjValue)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		kcSlice = append(kcSlice, kcObjValuable.(datasource_kubeconfig.KubeconfigsValue))
	}
	var diags diag.Diagnostics
	data.Kubeconfigs, diags = types.ListValueFrom(ctx, datasource_kubeconfig.KubeconfigsValue{}.Type(ctx), kcSlice)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func getBasicAuthToken() string {
	username := os.Getenv("DU_USERNAME")
	password := os.Getenv("DU_PASSWORD")
	jsonEncoded, err := json.Marshal(Credentials{
		Username: username,
		Password: password,
	})
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(jsonEncoded)
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type KubeConfig struct {
	APIVersion     string    `yaml:"apiVersion"`
	CurrentContext string    `yaml:"current-context"`
	Kind           string    `yaml:"kind"`
	Clusters       []Cluster `yaml:"clusters"`
	Contexts       []Context `yaml:"contexts"`
	Users          []User    `yaml:"users"`
}

type Cluster struct {
	Cluster struct {
		Server                   string `yaml:"server" tfsdk:"server"`
		CertificateAuthorityData string `yaml:"certificate-authority-data" tfsdk:"certificate_authority"`
	} `yaml:"cluster" tfsdk:"cluster"`
	Name string `yaml:"name" tfsdk:"name"`
}

type Context struct {
	Name    string `yaml:"name"`
	Context struct {
		Cluster   string `yaml:"cluster"`
		Namespace string `yaml:"namespace"`
		User      string `yaml:"user"`
	} `yaml:"context"`
}

type User struct {
	Name string `yaml:"name" tfsdk:"name"`
	User struct {
		Token                 string `yaml:"token" tfsdk:"token"`
		ClientCertificateData string `yaml:"client-certificate-data" tfsdk:"client_certificate"`
		ClientKeyData         string `yaml:"client-key-data" tfsdk:"client_key"`
	} `yaml:"user" tfsdk:"user"`
}
