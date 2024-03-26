package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/resource_cluster"

	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	// sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	"k8s.io/utils/ptr"
)

var _ resource.Resource = (*clusterResource)(nil)

func NewClusterResource() resource.Resource {
	return &clusterResource{}
}

type clusterResource struct {
	client *pmk.HTTPClient
}

func (r *clusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *clusterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_cluster.ClusterResourceSchema(ctx)
}

func (r *clusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*pmk.HTTPClient)
}

func (r *clusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data, state resource_cluster.ClusterModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create API call logic
	authInfo, err := r.client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID

	tflog.Debug(ctx, "Listing supported kubeRoleVersions for validations and defaulting to the latest")
	// https://platform9.com/docs/qbert/ref#getprovides-a-list-of-supported-pf9-kube-roles-for-a-cluster-
	supportedKubeRoleVersions, err := r.client.Qbert().ListSupportedVersions(authInfo.ProjectID)
	if err != nil {
		tflog.Error(ctx, "Failed to get supported versions", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get supported versions", err.Error())
		return
	}
	if !data.KubeRoleVersion.IsNull() && !data.KubeRoleVersion.IsUnknown() {
		tflog.Debug(ctx, "Validating if kube_role_version is supported", map[string]interface{}{"kube_role_version": data.KubeRoleVersion.ValueString()})
		// Validate using the response of
		allowedKubeRoleVersions := []string{}
		for _, role := range supportedKubeRoleVersions.Roles {
			allowedKubeRoleVersions = append(allowedKubeRoleVersions, role.RoleVersion)
		}
		if !StrSliceContains(allowedKubeRoleVersions, data.KubeRoleVersion.ValueString()) {
			// TODO: Do this in plan modifier or validate phase
			resp.Diagnostics.AddAttributeError(path.Root("kube_role_version"), "kube_role_version provided is not supported", fmt.Sprintf("Supported versions: %v", allowedKubeRoleVersions))
			return
		}
	}

	// if !data.AddonOperatorImageTag.IsNull() && !data.AddonOperatorImageTag.IsUnknown() {
	// 	// TODO: This API is not working as expected; it is returning 400 error
	// 	// validate using the response of: https://platform9.com/docs/qbert/ref#deleteget-all-supported-addon-operator-tags-for-a-pmk-cluster-as
	// 	supportedTags, err := r.client.Qbert().ListSupportedAddonOperatorTags(ctx, data.Id.ValueString())
	// 	if err != nil {
	// 		tflog.Error(ctx, "Failed to get supported addon operator tags", map[string]interface{}{"error": err})
	// 		resp.Diagnostics.AddError("Failed to get supported addon operator tags", err.Error())
	// 		return
	// 	}
	// 	if !StrSliceContains(supportedTags, data.AddonOperatorImageTag.ValueString()) {
	// 		resp.Diagnostics.AddAttributeError(path.Root("addon_operator_image_tag"), "addon_operator_image_tag provided is not supported", fmt.Sprintf("Supported tags: %v", supportedTags.Tags))
	// 		return
	// 	}
	// }

	createClusterReq, d := r.CreateCreateClusterRequest(ctx, authInfo.ProjectID, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to create create cluster object")
		return
	}

	if createClusterReq.KubeRoleVersion == "" {
		tflog.Debug(ctx, "Kube role version not provided")
		if len(supportedKubeRoleVersions.Roles) > 0 {
			latestKubeRoleVersion := findLatestKubeRoleVersion(supportedKubeRoleVersions.Roles)
			createClusterReq.KubeRoleVersion = latestKubeRoleVersion.RoleVersion
			tflog.Debug(ctx, "Using the latest kubeRoleVersion from supported versions", map[string]interface{}{"latestKubeRoleVersion": latestKubeRoleVersion.RoleVersion})
		} else {
			resp.Diagnostics.AddError("No supported kube role versions found", "No supported kube role versions found")
			return
		}
	}

	var addonsValues []resource_cluster.AddonsValue
	resp.Diagnostics.Append(data.Addons.ElementsAs(ctx, &addonsValues, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Mapping addons to addon-flags inside create cluster request")
	// qbert API is so complicated; some addons can be enabled through flags in create cluster request
	addonsValueMap := make(map[string]resource_cluster.AddonsValue, len(addonsValues))
	for _, addonValue := range addonsValues {
		addonsValueMap[addonValue.Name.ValueString()] = addonValue
		if addonValue.Enabled.IsNull() {
			tflog.Error(ctx, "Enabled field is required for addon", map[string]interface{}{"addon": addonValue})
			resp.Diagnostics.AddError("Enabled field is required for addon", fmt.Sprintf("addon: %v", addonValue))
			return
		}
		if addonValue.Enabled.IsUnknown() {
			tflog.Error(ctx, "Enabled field is unknown", map[string]interface{}{"addon": addonValue})
			resp.Diagnostics.AddError("Enabled field is unknown", fmt.Sprintf("addon: %v", addonValue))
			return
		}
		if addonValue.Enabled.ValueBool() {
			addonConfig := map[string]string{}
			if !addonValue.Config.IsNull() && !addonValue.Config.IsUnknown() {
				resp.Diagnostics.Append(addonValue.Config.ElementsAs(ctx, &addonConfig, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
			}
			switch addonValue.Name.ValueString() {
			case "kubevirt":
				createClusterReq.DeployKubevirt = true
			case "luigi":
				createClusterReq.DeployLuigiOperator = true
			case "metallb":
				createClusterReq.EnableMetalLb = true
				if metallbCidr, found := addonConfig["MetallbIpRange"]; found {
					createClusterReq.MetallbCidr = metallbCidr
				} else {
					resp.Diagnostics.AddAttributeError(path.Root("addons"), "MetallbIpRange is required for metallb addon", "MetallbIpRange is required for metallb addon")
					return
				}
			case "monitoring":
				monitoringConfig := qbert.MonitoringConfig{}
				addonConfig := map[string]string{}
				if retentionTime, found := addonConfig["retentionTime"]; found {
					monitoringConfig.RetentionTime = &retentionTime
				} else {
					monitoringConfig.RetentionTime = ptr.To("7d")
				}
				createClusterReq.Monitoring = &monitoringConfig
			case "pf9-profile-agent":
				createClusterReq.EnableProfileAgent = true
			case "something": //TODO: Find the actual name of the addon
				createClusterReq.EnableCatapultMonitoring = true
			default:
				tflog.Debug(ctx, "Addon cannot be enabled while create, it will be enabled after cluster creation", map[string]interface{}{"addon": addonValue})
			}
		}
	}

	// TODO: Add check for the level; marshalling is only required for logging purpose
	reqBody, err := json.Marshal(createClusterReq)
	if err != nil {
		tflog.Error(ctx, "Failed to marshal create cluster request", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to marshal create cluster request", err.Error())
		return
	}
	tflog.Info(ctx, "Creating a cluster in qbert", map[string]interface{}{"req": string(reqBody)})
	qbertClient := r.client.Qbert()
	clusterID, err := qbertClient.CreateCluster(*createClusterReq, projectID, qbert.CreateClusterOptions{})
	if err != nil {
		tflog.Error(ctx, "Failed to create cluster", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to create cluster", err.Error())
		return
	}

	nodeList := []qbert.Node{}
	var masterNodeIDs []string
	resp.Diagnostics.Append(data.MasterNodes.ElementsAs(ctx, &masterNodeIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, nodeID := range masterNodeIDs {
		nodeList = append(nodeList, qbert.Node{
			UUID:     nodeID,
			IsMaster: 1,
		})
	}
	var workerNodeIDs []string
	resp.Diagnostics.Append(data.WorkerNodes.ElementsAs(ctx, &workerNodeIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, nodeID := range workerNodeIDs {
		nodeList = append(nodeList, qbert.Node{
			UUID:     nodeID,
			IsMaster: 0,
		})
	}
	tflog.Info(ctx, "Attaching nodes", map[string]interface{}{"nodeList": nodeList})
	err = qbertClient.AttachNodes(clusterID, nodeList)
	if err != nil {
		tflog.Error(ctx, "Failed to attach nodes", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to attach nodes", err.Error())
		return
	}
	// TODO: Should we save an intermediate state between multiple requests?

	// The sunpike/qbert auto enables some of the addons; so we need to reconcile the state.
	// i.e, match the state with the plan if there are differences
	// For example, is the practitioner has provided config for coredns addon, we have to call
	// sunpike api to get it configured as per plan
	tflog.Debug(ctx, "Getting list of enabled addons including that are enabled by the backend")
	addons, err := r.client.Qbert().ListClusterAddons(fmt.Sprintf("sunpike.pf9.io/cluster=%s", clusterID))
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster addons", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
		return
	}
	tflog.Debug(ctx, "Got list of enabled addons", map[string]interface{}{"addons": addons})
	// reconcileAddons() is common across Create() and Update() to avoid code duplication.
	// It expects addons in the map[string]resource_cluster.AddonsValue format, where key is addon name.
	// So, we need to convert the list of addons to map.
	stateAddonsValueMap := make(map[string]resource_cluster.AddonsValue, len(addons.Items))
	for _, addon := range addons.Items {
		addonValue := resource_cluster.AddonsValue{}
		addonValue.Name = types.StringValue(addon.Spec.Type)
		addonValue.Enabled = types.BoolValue(true)
		addonValue.Version = types.StringValue(addon.Spec.Version)
		addonConfig := map[string]string{}
		for _, param := range addon.Spec.Override.Params {
			addonConfig[param.Name] = param.Value
		}
		var diags diag.Diagnostics
		addonValue.Config, diags = types.MapValueFrom(ctx, basetypes.StringType{}, addonConfig)
		if diags.HasError() {
			return
		}
		// TODO: Use recommended way if someone responds to the issue:
		// https://discuss.hashicorp.com/t/using-terraform-plugin-codegen-framework-generated-code-to-instantiate-nested-objects/64026
		stateAddonsValueMap[addon.Spec.Type] = resource_cluster.NewKnownAddonsValueMust(addonValue)
	}
	resp.Diagnostics.Append(r.reconcileAddons(ctx, addonsValueMap, stateAddonsValueMap)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.readStateFromRemote(ctx, clusterID, projectID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// reconcileAddons compares the plan and state of addons and updates the addons in the backend if needed
func (r *clusterResource) reconcileAddons(ctx context.Context, plan map[string]resource_cluster.AddonsValue, state map[string]resource_cluster.AddonsValue) diag.Diagnostics {
	var diags diag.Diagnostics
	for addonName, planAddon := range plan {
		if stateAddon, found := state[addonName]; found {
			if planAddon.Enabled.IsNull() {
				tflog.Error(ctx, "Enabled field is required for addon", map[string]interface{}{"addon": addonName})
				diags.AddAttributeError(path.Root("addons"), "Enabled field is required for addon", fmt.Sprintf("addon: %v", addonName))
				return diags
			}
			if planAddon.Enabled.IsUnknown() {
				tflog.Error(ctx, "Enabled field is unknown", map[string]interface{}{"addon": addonName})
				diags.AddAttributeError(path.Root("addons"), "Enabled field is required for addon", fmt.Sprintf("addon: %v", addonName))
				return diags
			}
			if planAddon.Enabled.ValueBool() {
				// TODO: Use https://platform9.com/docs/qbert/ref#deleteget-all-supported-cluster-addon-versions-for-a-pmk-cluster
				// to get the supported versions of the addon; raise error if the version in the plan is not supported
				diags.Append(r.patchAddon(ctx, planAddon, stateAddon)...)
				if diags.HasError() {
					return diags
				}
			} else {
				tflog.Debug(ctx, "Disabling addon", map[string]interface{}{"addon": addonName})
				// TODO: Implement delete addon
				// err = r.client.Sunpike().Delete(ctx, &addon)
				// if err != nil {
				// 	tflog.Error(ctx, "Failed to disable addon", map[string]interface{}{"error": err})
				// 	diags.AddError("Failed to disable addon", err.Error())
				// 	return diags
				// }
			}
		} else {
			// addon available in plan but not in state
			if planAddon.Enabled.IsNull() {
				tflog.Error(ctx, "Enabled field is required for addon", map[string]interface{}{"addon": addonName})
				diags.AddAttributeError(path.Root("addons"), "Enabled field is required for addon", fmt.Sprintf("addon: %v", addonName))
				return diags
			}
			if planAddon.Enabled.IsUnknown() {
				tflog.Error(ctx, "Enabled field is unknown", map[string]interface{}{"addon": addonName})
				diags.AddAttributeError(path.Root("addons"), "Enabled field is required for addon", fmt.Sprintf("addon: %v", addonName))
				return diags
			}
			if planAddon.Enabled.ValueBool() {
				tflog.Debug(ctx, "Enabling addon", map[string]interface{}{"addon": addonName})
				// TODO: Implement create addon
				err := r.client.Sunpike().Create(ctx, &sunpikev1alpha2.ClusterAddon{
					Spec: sunpikev1alpha2.ClusterAddonSpec{
						Type:    addonName,
						Version: planAddon.Version.ValueString(),
						Override: sunpikev1alpha2.Override{
							Params: []sunpikev1alpha2.Params{},
						},
					},
				})
				if err != nil {
					tflog.Error(ctx, "Failed to enable addon", map[string]interface{}{"error": err})
					diags.AddError("Failed to enable addon", err.Error())
					return diags
				}
			}
		}
	}
	return diags
}

// patchAddon patches Addon using sunpike API, patch includes changing overrides and version
func (r *clusterResource) patchAddon(ctx context.Context, plan resource_cluster.AddonsValue, state resource_cluster.AddonsValue) diag.Diagnostics {
	var diags diag.Diagnostics
	var planConfig, stateConfig map[string]string
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		diags = plan.Config.ElementsAs(ctx, &planConfig, false)
		if diags.HasError() {
			return diags
		}
	}
	diags = state.Config.ElementsAs(ctx, &stateConfig, false)
	if diags.HasError() {
		return diags
	}
	var patchRequired bool
	var params []sunpikev1alpha2.Params
	for key, planValue := range planConfig {
		if stateValue, found := stateConfig[key]; found && stateValue == planValue {
			// No need to patch
			continue
		}
		params = append(params, sunpikev1alpha2.Params{
			Name:  key,
			Value: planValue,
		})
		patchRequired = true
	}
	type PatchBody struct {
		Spec struct {
			Override sunpikev1alpha2.Override `json:"override,omitempty"`
			Version  string                   `json:"version,omitempty"`
		} `json:"spec,omitempty"`
	}
	var patchBody PatchBody
	patchBody.Spec.Override.Params = params
	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		if !plan.Version.Equal(state.Version) {
			patchBody.Spec.Version = plan.Version.ValueString()
			patchRequired = true
		}
	}
	if patchRequired {
		jsonBody, err := json.Marshal(patchBody)
		if err != nil {
			tflog.Error(ctx, "Failed to marshal patch body", map[string]interface{}{"error": err})
			diags.AddError("Failed to marshal patch body", err.Error())
			return diags
		}
		//`{"spec: {"override":{"params":[{"name":"retentionTime","value":"7d"}]},"version":"0.68.0"}`
		tflog.Debug(ctx, "TODO: Patching addon", map[string]interface{}{"addon": plan.Name.ValueString(), "patchBody": string(jsonBody)})
		// TODO: Update the addon config using patch
		// TODO: ?? Does setting addonversion here will work or we need to call https://platform9.com/docs/qbert/ref#postupgrade-a-cluster-identified-by-the-uuid
		// r.client.Sunpike().Patch(ctx, &addon, patchBody)
		return diags
	}
	return diags
}

func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data, state resource_cluster.ClusterModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	authInfo, err := r.client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	clusterID := data.Id.ValueString()
	projectID := authInfo.ProjectID
	resp.Diagnostics.Append(r.readStateFromRemote(ctx, clusterID, projectID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated state into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state resource_cluster.ClusterModel

	// Read Terraform plan plan into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update API call logic
	authInfo, err := r.client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID
	clusterID := state.Id.ValueString()
	if !plan.WorkerNodes.Equal(state.WorkerNodes) || !plan.MasterNodes.Equal(state.MasterNodes) {
		tflog.Debug(ctx, "Change in nodes detected, attaching/detaching nodes")
		resp.Diagnostics.Append(r.attachDetachNodes(ctx, plan, state)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// if !plan.AddonOperatorImageTag.IsNull() && !plan.AddonOperatorImageTag.IsUnknown() {
	// 	// TODO: This API is not working as expected; it is returning 400 error
	// 	// validate using the response of: https://platform9.com/docs/qbert/ref#deleteget-all-supported-addon-operator-tags-for-a-pmk-cluster-as
	// 	supportedTags, err := r.client.Qbert().ListSupportedAddonOperatorTags(ctx, clusterID)
	// 	if err != nil {
	// 		tflog.Error(ctx, "Failed to get supported addon operator tags", map[string]interface{}{"error": err})
	// 		resp.Diagnostics.AddError("Failed to get supported addon operator tags", err.Error())
	// 		return
	// 	}
	// 	if !StrSliceContains(supportedTags, plan.AddonOperatorImageTag.ValueString()) {
	// 		resp.Diagnostics.AddAttributeError(path.Root("addon_operator_image_tag"), "addon_operator_image_tag provided is not supported", fmt.Sprintf("Supported tags: %v", supportedTags.Tags))
	// 		return
	// 	}
	// }

	editClusterReq := qbert.EditClusterRequest{}
	var editRequired bool
	if !plan.EtcdBackup.Equal(state.EtcdBackup) {
		editRequired = true
		var etcdConfig qbert.EtcdBackupConfig
		etcdConfig.DailyBackupTime = plan.EtcdBackup.DailyBackupTime.ValueString()
		if plan.EtcdBackup.IsEtcdBackupEnabled.ValueBool() {
			etcdConfig.IsEtcdBackupEnabled = 1
		} else {
			etcdConfig.IsEtcdBackupEnabled = 0
		}
		etcdConfig.MaxTimestampBackupCount = int(plan.EtcdBackup.MaxTimestampBackupCount.ValueInt64())
		etcdConfig.StorageProperties.LocalPath = plan.EtcdBackup.StorageLocalPath.ValueStringPointer()
		etcdConfig.StorageType = plan.EtcdBackup.StorageType.ValueString()

		etcdConfig.IntervalInHours = int(plan.EtcdBackup.IntervalInHours.ValueInt64())
		etcdConfig.IntervalInMins = int(plan.EtcdBackup.IntervalInMins.ValueInt64())
		etcdConfig.MaxIntervalBackupCount = int(plan.EtcdBackup.MaxIntervalBackupCount.ValueInt64())
		editClusterReq.EtcdBackup = &etcdConfig
	}

	// TODO: The following attrributes are not in the swagger: https://platform9.com/docs/qbert/ref#putupdate-the-properties-of-a-cluster-specified-by-the-cluster-u
	// but available in the qbert server code: https://github.com/platform9/pf9-qbert/blob/d23bce9c9cb64caf786a561627c8d0b98dfdbc7c/server/api/handlers/v1/index.js#L267
	if !plan.CertExpiryHrs.Equal(state.CertExpiryHrs) {
		editRequired = true
		editClusterReq.CertExpiryHrs = int(plan.CertExpiryHrs.ValueInt64())
	}

	if !plan.ContainersCidr.Equal(state.ContainersCidr) {
		editRequired = true
		editClusterReq.ContainersCidr = plan.ContainersCidr.ValueString()
	}
	if !plan.ServicesCidr.Equal(state.ServicesCidr) {
		editRequired = true
		editClusterReq.ServicesCidr = plan.ServicesCidr.ValueString()
	}

	// TODO: Add following fields are in provider_code_spec.json; if needed
	// editClusterReq.KubeProxyMode = plan.KubeProxyMode.ValueString()
	// editClusterReq.EnableProfileAgent = plan.EnableProfileAgent.ValueBool()
	// editClusterReq.EnableCatapultMonitoring = plan.EnableCatapultMonitoring.ValueBool()

	if editRequired {
		// qberty API replaces tags with empty map if tags are not provided
		editClusterReq.Tags = map[string]string{}
		tagsGoMap := map[string]string{}
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tagsGoMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		editClusterReq.Tags = tagsGoMap

		err = r.client.Qbert().EditCluster(editClusterReq, clusterID, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update cluster", err.Error())
			return
		}
	} else {
		tflog.Debug(ctx, "No change detected, skipping update")
	}

	var upgradeRequired bool
	var upgradeClusterReq qbert.UpgradeClusterRequest
	if !plan.KubeRoleVersion.Equal(state.KubeRoleVersion) {
		tflog.Info(ctx, "upgrading kube role version", map[string]interface{}{"from": state.KubeRoleVersion, "to": plan.KubeRoleVersion})
		supportedVersions, err := r.client.Qbert().ListSupportedVersions(projectID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to find kube role versions for a cluster", err.Error())
			return
		}
		var upgradeAllowed bool
		for _, version := range supportedVersions.Roles {
			if version.RoleVersion == plan.KubeRoleVersion.ValueString() {
				upgradeAllowed = true
				break
			}
		}
		if !upgradeAllowed {
			resp.Diagnostics.AddAttributeError(path.Root("kube_role_version"), "Kube role version cannot be upgraded to this version", fmt.Sprintf("Allowed versions are: %v", supportedVersions.Roles))
			return
		}
		// TODO: https://platform9.com/docs/qbert/ref#postupgrade-a-cluster-identified-by-the-uuid /upgrade?type=minor query param not documented
		// TODO:?? What to sent with batchUpgradePercent or batchUpgradeNodes ??
		upgradeRequired = true
		planVersionRole := qbert.ParseVersion(plan.KubeRoleVersion.ValueString())
		stateVersionRole := qbert.ParseVersion(state.KubeRoleVersion.ValueString())
		upgradeClusterReq.KubeRoleVersionUpgradeType = planVersionRole.UpgradeType(stateVersionRole)
	}

	// TODO:?? Ask PMK team if the same function works for the following upgrades, because source code does not contain anythign related to these
	// https://github.com/platform9/pf9-qbert/blob/d23bce9c9cb64caf786a561627c8d0b98dfdbc7c/server/api/handlers/v4/index.js#L592
	if !plan.ContainerRuntime.Equal(state.ContainerRuntime) {
		upgradeRequired = true
		upgradeClusterReq.ContainerRuntime = plan.ContainerRuntime.ValueString()
	}
	if !plan.AddonOperatorImageTag.Equal(state.AddonOperatorImageTag) {
		upgradeRequired = true
		upgradeClusterReq.AddonOperatorImageTag = plan.AddonOperatorImageTag.ValueString()
	}
	if !plan.CustomRegistryUrl.Equal(state.CustomRegistryUrl) {
		upgradeRequired = true
		upgradeClusterReq.CustomRegistryUrl = plan.CustomRegistryUrl.ValueString()
	}
	if !plan.CustomRegistryRepoPath.Equal(state.CustomRegistryRepoPath) {
		upgradeRequired = true
		upgradeClusterReq.CustomRegistryRepoPath = plan.CustomRegistryRepoPath.ValueString()
	}
	if !plan.CustomRegistryUsername.Equal(state.CustomRegistryUsername) {
		upgradeRequired = true
		upgradeClusterReq.CustomRegistryUsername = plan.CustomRegistryUsername.ValueString()
	}
	if !plan.CustomRegistryPassword.Equal(state.CustomRegistryPassword) {
		upgradeRequired = true
		upgradeClusterReq.CustomRegistryPassword = plan.CustomRegistryPassword.ValueString()
	}
	if !plan.CustomRegistrySkipTls.Equal(state.CustomRegistrySkipTls) {
		upgradeRequired = true
		upgradeClusterReq.CustomRegistrySkipTls = plan.CustomRegistrySkipTls.ValueBool()
	}
	if !plan.CustomRegistrySelfSignedCerts.Equal(state.CustomRegistrySelfSignedCerts) {
		upgradeRequired = true
		upgradeClusterReq.CustomRegistrySelfSignedCerts = plan.CustomRegistrySelfSignedCerts.ValueBool()
	}
	if !plan.CustomRegistryCertPath.Equal(state.CustomRegistryCertPath) {
		upgradeRequired = true
		upgradeClusterReq.CustomRegistryCertPath = plan.CustomRegistryCertPath.ValueString()
	}
	// TODO: ?? Should we add addonVersions to upgradeClusterReq; because it can be upgraded using sunpike api anyways
	if upgradeRequired {
		err = r.client.Qbert().UpgradeCluster(ctx, upgradeClusterReq, clusterID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to upgrade cluster", err.Error())
			return
		}
	}
	// ?? Addonversion is available in both sunpike get all addons as well as
	// https://platform9.com/docs/qbert/ref#getprovides-a-list-of-addon-version-for-pf9-kube-role-on-a-clust
	// Which one to use?

	var planAddons []resource_cluster.AddonsValue
	resp.Diagnostics.Append(plan.Addons.ElementsAs(ctx, &planAddons, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	planAddonsMap := make(map[string]resource_cluster.AddonsValue, len(planAddons))
	for _, planAddon := range planAddons {
		planAddonsMap[planAddon.Name.ValueString()] = planAddon
	}
	var stateAddons []resource_cluster.AddonsValue
	resp.Diagnostics.Append(state.Addons.ElementsAs(ctx, &stateAddons, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	stateAddonsMap := map[string]resource_cluster.AddonsValue{}
	for _, stateAddon := range stateAddonsMap {
		stateAddonsMap[stateAddon.Name.ValueString()] = stateAddon
	}
	resp.Diagnostics.Append(r.reconcileAddons(ctx, planAddonsMap, stateAddonsMap)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.readStateFromRemote(ctx, clusterID, projectID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// readStateFromRemote sets the values of the attibutes in the state variable retrieved from the backend
func (r *clusterResource) readStateFromRemote(ctx context.Context, clusterID, projectID string, state *resource_cluster.ClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics

	tflog.Info(ctx, "Reading cluster from qbert", map[string]interface{}{"clusterID": clusterID})
	cluster, err := r.client.Qbert().GetCluster(ctx, projectID, clusterID)
	if err != nil {
		diags.AddError("Failed to get cluster", err.Error())
		return diags
	}
	diags = qbertClusterToTerraformCluster(ctx, cluster, state)
	if diags.HasError() {
		return diags
	}

	tflog.Info(ctx, "Listing nodes attached to the cluster", map[string]interface{}{"clusterID": clusterID})
	clusterNodes, err := r.client.Qbert().ListClusterNodes(ctx, clusterID)
	if err != nil {
		diags.AddError("Failed to get cluster nodes", err.Error())
		return diags
	}
	masterNodes := []string{}
	workerNodes := []string{}
	for _, node := range clusterNodes {
		if node.IsMaster == 1 {
			masterNodes = append(masterNodes, node.UUID)
		} else {
			workerNodes = append(workerNodes, node.UUID)
		}
	}
	state.MasterNodes, diags = types.SetValueFrom(ctx, basetypes.StringType{}, masterNodes)
	if diags.HasError() {
		return diags
	}
	state.WorkerNodes, diags = types.SetValueFrom(ctx, basetypes.StringType{}, workerNodes)
	if diags.HasError() {
		return diags
	}

	tflog.Info(ctx, "Listing addons enabled on the cluster", map[string]interface{}{"clusterID": clusterID})
	addons, err := r.client.Qbert().ListClusterAddons(fmt.Sprintf("sunpike.pf9.io/cluster=%s", clusterID))
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster addons", map[string]interface{}{"error": err})
		diags.AddError("Failed to get cluster addons", err.Error())
		return diags
	}
	addonsSlice := []resource_cluster.AddonsValue{}
	for _, addon := range addons.Items {
		addonValue := resource_cluster.AddonsValue{}
		addonValue.Name = types.StringValue(addon.Spec.Type)
		addonValue.Enabled = types.BoolValue(true)
		addonValue.Version = types.StringValue(addon.Spec.Version)
		addonConfig := map[string]string{}
		for _, param := range addon.Spec.Override.Params {
			addonConfig[param.Name] = param.Value
		}
		addonValue.Config, diags = types.MapValueFrom(ctx, basetypes.StringType{}, addonConfig)
		if diags.HasError() {
			return diags
		}

		// Conversion from AddonsValue{} to ObjectValue{} and then to ObjectValuable and finally
		// back to AddonsValue{} with known state.

		// TODO: Use recommended way if someone responds to the issue:
		// https://discuss.hashicorp.com/t/using-terraform-plugin-codegen-framework-generated-code-to-instantiate-nested-objects/64026
		objectValue, diags := addonValue.ToObjectValue(ctx)
		if diags.HasError() {
			return diags
		}
		addonObjValuable, diags := resource_cluster.AddonsType{}.ValueFromObject(ctx, objectValue)
		diags.Append(diags...)
		if diags.HasError() {
			return diags
		}
		addonValue = addonObjValuable.(resource_cluster.AddonsValue)
		addonsSlice = append(addonsSlice, addonValue)
	}
	state.Addons, diags = types.SetValueFrom(ctx, resource_cluster.AddonsValue{}.Type(ctx), addonsSlice)
	if diags.HasError() {
		return diags
	}
	return diags
}

func (r *clusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data resource_cluster.ClusterModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete API call logic
	authInfo, err := r.client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}

	projectID := authInfo.ProjectID
	clusterID := data.Id.ValueString()
	err = r.client.Qbert().DeleteCluster(clusterID, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete cluster", err.Error())
		return
	}
}

func (r *clusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func qbertClusterToTerraformCluster(ctx context.Context, in *qbert.Cluster, out *resource_cluster.ClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics

	out.Id = types.StringValue(in.UUID)
	out.Name = types.StringValue(in.Name)
	out.AllowWorkloadsOnMaster = types.BoolValue(in.AllowWorkloadsOnMaster != 0)
	out.MasterIp = types.StringValue(in.MasterIp)
	out.MasterVipIface = types.StringValue(in.MasterVipIface)
	out.MasterVipIpv4 = types.StringValue(in.MasterVipIpv4)
	out.ContainersCidr = types.StringValue(in.ContainersCidr)
	out.ServicesCidr = types.StringValue(in.ServicesCidr)
	mtuSizeInt, err := strconv.Atoi(in.MtuSize)
	if err != nil {
		tflog.Error(ctx, "Failed to parse mtu size", map[string]interface{}{"error": err})
		diags.AddError("Failed to parse mtu size", err.Error())
		return diags
	}
	out.MtuSize = types.Int64Value(int64(mtuSizeInt))
	out.Privileged = types.BoolValue(in.Privileged != 0)
	out.UseHostname = types.BoolValue(in.UseHostname)
	out.InterfaceDetectionMethod = types.StringValue(in.InterfaceDetectionMethod)
	out.InterfaceName = types.StringValue(in.InterfaceName)
	out.NodePoolUuid = types.StringValue(in.NodePoolUuid)
	out.KubeRoleVersion = types.StringValue(in.KubeRoleVersion)
	out.CpuManagerPolicy = types.StringValue(in.CPUManagerPolicy)
	out.TopologyManagerPolicy = types.StringValue(in.TopologyManagerPolicy)
	out.CalicoIpIpMode = types.StringValue(in.CalicoIpIpMode)
	out.CalicoNatOutgoing = types.BoolValue(in.CalicoNatOutgoing != 0)
	out.CalicoV4BlockSize = types.StringValue(in.CalicoV4BlockSize)
	out.CalicoIpv4DetectionMethod = types.StringValue(in.CalicoIPv4DetectionMethod)
	out.NetworkPlugin = types.StringValue(in.NetworkPlugin)
	out.ContainerRuntime = types.StringValue(in.ContainerRuntime)
	out.RuntimeConfig = types.StringValue(in.RuntimeConfig)

	out.ExternalDnsName = types.StringValue(in.ExternalDnsName)
	out.CertExpiryHrs = types.Int64Value(int64(in.CertExpiryHrs))
	out.CalicoNodeCpuLimit = types.StringValue(in.CalicoNodeCpuLimit)
	out.CalicoNodeMemoryLimit = types.StringValue(in.CalicoNodeMemoryLimit)
	out.CalicoTyphaCpuLimit = types.StringValue(in.CalicoTyphaCpuLimit)
	out.CalicoTyphaMemoryLimit = types.StringValue(in.CalicoTyphaMemoryLimit)
	out.CalicoControllerCpuLimit = types.StringValue(in.CalicoControllerCpuLimit)
	out.CalicoControllerMemoryLimit = types.StringValue(in.CalicoControllerMemoryLimit)

	// Computed attributes
	out.CreatedAt = types.StringValue(in.CreatedAt)
	out.Status = types.StringValue(in.Status)
	out.FlannelIfaceLabel = types.StringValue(in.FlannelIfaceLabel)
	out.FlannelPublicIfaceLabel = types.StringValue(in.FlannelPublicIfaceLabel)
	out.DockerRoot = types.StringValue(in.DockerRoot)
	out.EtcdDataDir = types.StringValue(in.EtcdDataDir)
	out.LastOp = types.StringValue(in.LastOp)
	out.LastOk = types.StringValue(in.LastOk)
	out.TaskStatus = types.StringValue(in.TaskStatus)
	out.TaskError = types.StringValue(in.TaskError)
	out.ProjectId = types.StringValue(in.ProjectId)
	out.MasterVipVrouterId = types.StringValue(in.MasterVipVrouterId)
	out.K8sApiPort = types.StringValue(in.K8sApiPort)
	out.CalicoIpv4 = types.StringValue(in.CalicoIPv4)
	out.CalicoIpv6 = types.StringValue(in.CalicoIPv6)
	out.CalicoIpv6DetectionMethod = types.StringValue(in.CalicoIPv6DetectionMethod)
	out.CalicoRouterId = types.StringValue(in.CalicoRouterID)
	out.CalicoIpv6PoolCidr = types.StringValue(in.CalicoIPv6PoolCidr)
	out.CalicoIpv6PoolBlockSize = types.StringValue(in.CalicoIPv6PoolBlockSize)
	out.CalicoIpv6PoolNatOutgoing = types.BoolValue(in.CalicoIPv6PoolNatOutgoing != 0)
	out.FelixIpv6Support = types.BoolValue(in.FelixIPv6Support != 0)
	out.Masterless = types.BoolValue(in.Masterless != 0)
	out.EtcdVersion = types.StringValue(in.EtcdVersion)
	if in.EtcdHeartbeatIntervalMs == "" {
		out.EtcdHeartbeatIntervalMs = types.Int64Null()
	} else {
		etcdHeartbeatIntervalMs, err := strconv.Atoi(in.EtcdHeartbeatIntervalMs)
		if err != nil {
			tflog.Error(ctx, "Failed to parse etcd heartbeat interval", map[string]interface{}{"error": err})
			diags.AddError("Failed to parse etcd heartbeat interval", err.Error())
			return diags
		}
		out.EtcdHeartbeatIntervalMs = types.Int64Value(int64(etcdHeartbeatIntervalMs))
	}
	if in.EtcdElectionTimeoutMs == "" {
		out.EtcdElectionTimeoutMs = types.Int64Null()
	} else {
		etcdElectionTimeoutMs, err := strconv.Atoi(in.EtcdElectionTimeoutMs)
		if err != nil {
			tflog.Error(ctx, "Failed to parse etcd election timeout", map[string]interface{}{"error": err})
			diags.AddError("Failed to parse etcd election timeout", err.Error())
			return diags
		}
		out.EtcdElectionTimeoutMs = types.Int64Value(int64(etcdElectionTimeoutMs))
	}
	out.MasterStatus = types.StringValue(in.MasterStatus)
	out.WorkerStatus = types.StringValue(in.WorkerStatus)
	out.Ipv6 = types.BoolValue(in.IPv6 != 0)
	out.NodePoolName = types.StringValue(in.NodePoolName)
	out.CloudProviderUuid = types.StringValue(in.CloudProviderUuid)
	out.CloudProviderName = types.StringValue(in.CloudProviderName)
	out.CloudProviderType = types.StringValue(in.CloudProviderType)
	out.DockerPrivateRegistry = types.StringValue(in.DockerPrivateRegistry)
	out.QuayPrivateRegistry = types.StringValue(in.QuayPrivateRegistry)
	out.GcrPrivateRegistry = types.StringValue(in.GcrPrivateRegistry)
	out.K8sPrivateRegistry = types.StringValue(in.K8sPrivateRegistry)
	out.DockerCentosPackageRepoUrl = types.StringValue(in.DockerCentosPackageRepoUrl)
	out.DockerUbuntuPackageRepoUrl = types.StringValue(in.DockerUbuntuPackageRepoUrl)
	out.AddonOperatorImageTag = types.StringValue(in.AddonOperatorImageTag)
	out.InterfaceReachableIp = types.StringValue(in.InterfaceReachableIP)
	out.CustomRegistryUrl = types.StringValue(in.CustomRegistryUrl)
	out.CustomRegistryRepoPath = types.StringValue(in.CustomRegistryRepoPath)
	out.CustomRegistryUsername = types.StringValue(in.CustomRegistryUsername)
	out.CustomRegistryPassword = types.StringValue(in.CustomRegistryPassword)
	out.CustomRegistrySkipTls = types.BoolValue(in.CustomRegistrySkipTls != 0)
	out.CustomRegistrySelfSignedCerts = types.BoolValue(in.CustomRegistrySelfSignedCerts != 0)
	out.CustomRegistryCertPath = types.StringValue(in.CustomRegistryCertPath)

	if in.EnableEtcdEncryption == "true" {
		out.EnableEtcdEncryption = types.BoolValue(true)
	} else {
		out.EnableEtcdEncryption = types.BoolValue(false)
	}
	if in.EtcdBackup != nil {
		var localPathVal types.String
		storageProps := in.EtcdBackup.StorageProperties
		if storageProps.LocalPath != nil {
			localPathVal = types.StringValue(*in.EtcdBackup.StorageProperties.LocalPath)
		} else {
			localPathVal = types.StringNull()
		}
		etcdBackup, d := resource_cluster.NewEtcdBackupValue(
			resource_cluster.EtcdBackupValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"is_etcd_backup_enabled":     types.BoolValue(in.EtcdBackup.IsEtcdBackupEnabled != 0),
				"storage_type":               types.StringValue(in.EtcdBackup.StorageType),
				"max_timestamp_backup_count": getIntOrNullIfZero(in.EtcdBackup.MaxTimestampBackupCount),
				"storage_local_path":         localPathVal,
				"daily_backup_time":          getStrOrNullIfEmpty(in.EtcdBackup.DailyBackupTime),
				"interval_in_hours":          getIntOrNullIfZero(in.EtcdBackup.IntervalInHours),
				"interval_in_mins":           getIntOrNullIfZero(in.EtcdBackup.IntervalInMins),
				"max_interval_backup_count":  getIntOrNullIfZero(in.EtcdBackup.MaxIntervalBackupCount),
			},
		)
		if d.HasError() {
			return d
		}
		out.EtcdBackup = etcdBackup
	}
	if len(in.Tags) == 0 {
		out.Tags = types.MapNull(basetypes.StringType{})
	} else {
		tagsGoMap := map[string]attr.Value{}
		for key, val := range in.Tags {
			tagsGoMap[key] = types.StringValue(val)
		}
		tfMap, d := types.MapValueFrom(ctx, types.StringType, tagsGoMap)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		out.Tags = tfMap
	}

	return diags
}

func (r *clusterResource) CreateCreateClusterRequest(ctx context.Context, projectID string, in *resource_cluster.ClusterModel) (*qbert.CreateClusterRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := getDefaultCreateClusterReq()
	req.Name = in.Name.ValueString()
	req.Privileged = in.Privileged.ValueBoolPointer()
	req.MasterIP = in.MasterIp.ValueString()
	masterNodes := []string{}
	diags.Append(in.MasterNodes.ElementsAs(ctx, &masterNodes, false)...)
	if diags.HasError() {
		return nil, diags
	}
	req.MasterNodes = masterNodes

	if !in.WorkerNodes.IsNull() && !in.WorkerNodes.IsUnknown() {
		// if allow_workloads_on_master is true, then we dont need to add worker nodes
		workerNodes := []string{}
		diags.Append(in.WorkerNodes.ElementsAs(ctx, &workerNodes, false)...)
		if diags.HasError() {
			return nil, diags
		}
		req.WorkerNodes = workerNodes
		if areNotMutuallyExclusive(masterNodes, workerNodes) {
			diags.AddAttributeError(path.Root("worker_nodes"), "worker_nodes and master_nodes should be mutually exclusive", "Same node can not be part of both worker and master nodes")
			return nil, diags
		}
	}
	req.AllowWorkloadOnMaster = in.AllowWorkloadsOnMaster.ValueBoolPointer()
	req.MasterVirtualIPIface = in.MasterVipIface.ValueString()
	req.MasterVirtualIP = in.MasterVipIpv4.ValueString()
	req.ContainerCIDR = in.ContainersCidr.ValueString()
	req.ServiceCIDR = in.ServicesCidr.ValueString()
	req.MTUSize = ptr.To(int(in.MtuSize.ValueInt64()))
	req.Privileged = in.Privileged.ValueBoolPointer()
	req.UseHostname = in.UseHostname.ValueBoolPointer()
	if !in.InterfaceDetectionMethod.IsUnknown() {
		req.InterfaceDetectionMethod = in.InterfaceDetectionMethod.ValueString()
	} else {
		if len(req.WorkerNodes) > 0 {
			// For non SingleNode clusters the default InterfaceDetectionMethod should be FirstFound as per UI
			// Non single node clusters are the Multi Master and Single Master clusters
			req.InterfaceDetectionMethod = "FirstFound"
		}
	}
	req.InterfaceName = in.InterfaceName.ValueString()
	if in.NodePoolUuid.IsNull() || in.NodePoolUuid.IsUnknown() || in.NodePoolUuid.ValueString() == "" {
		tflog.Debug(ctx, "Node pool UUID not provided, getting default node pool")
		localNodePoolUUID, err := r.client.Qbert().GetNodePoolID(projectID)
		if err != nil {
			tflog.Error(ctx, "Failed to get node pool", map[string]interface{}{"error": err})
			diags.AddError("Failed to get node pool", err.Error())
			return nil, diags
		}
		tflog.Debug(ctx, "Got default node pool", map[string]interface{}{"nodePoolUUID": localNodePoolUUID})
		req.NodePoolUUID = localNodePoolUUID
	} else {
		tflog.Debug(ctx, "Node pool UUID provided", map[string]interface{}{"nodePoolUUID": in.NodePoolUuid.ValueString()})
		req.NodePoolUUID = in.NodePoolUuid.ValueString()
	}
	req.KubeRoleVersion = in.KubeRoleVersion.ValueString()
	req.CPUManagerPolicy = in.CpuManagerPolicy.ValueString()
	req.ExternalDNSName = in.ExternalDnsName.ValueString()
	req.TopologyManagerPolicy = in.TopologyManagerPolicy.ValueString()
	req.CalicoIPIPMode = in.CalicoIpIpMode.ValueString()
	req.CalicoNatOutgoing = in.CalicoNatOutgoing.ValueBoolPointer()
	req.CalicoV4BlockSize = in.CalicoV4BlockSize.ValueString()
	req.CalicoIpv4DetectionMethod = in.CalicoIpv4DetectionMethod.ValueString()
	req.NetworkPlugin = qbert.CNIBackend(in.NetworkPlugin.ValueString())
	req.RuntimeConfig = in.RuntimeConfig.ValueString()
	req.ContainerRuntime = qbert.ContainerRuntime(in.ContainerRuntime.ValueString())

	if !in.EnableEtcdEncryption.IsUnknown() {
		req.EnableEtcdEncryption = fmt.Sprintf("%v", in.EnableEtcdEncryption.ValueBool())
	}
	req.EtcdBackup.DailyBackupTime = in.EtcdBackup.DailyBackupTime.ValueString()
	if in.EtcdBackup.IsEtcdBackupEnabled.ValueBool() {
		req.EtcdBackup.IsEtcdBackupEnabled = 1
	} else {
		req.EtcdBackup.IsEtcdBackupEnabled = 0
	}
	req.EtcdBackup.MaxTimestampBackupCount = int(in.EtcdBackup.MaxTimestampBackupCount.ValueInt64())
	req.EtcdBackup.StorageProperties.LocalPath = in.EtcdBackup.StorageLocalPath.ValueStringPointer()
	req.EtcdBackup.StorageType = in.EtcdBackup.StorageType.ValueString()
	req.EtcdBackup.IntervalInHours = int(in.EtcdBackup.IntervalInHours.ValueInt64())
	req.EtcdBackup.IntervalInMins = int(in.EtcdBackup.IntervalInMins.ValueInt64())
	if !in.EtcdBackup.MaxIntervalBackupCount.IsUnknown() {
		req.EtcdBackup.MaxIntervalBackupCount = int(in.EtcdBackup.MaxIntervalBackupCount.ValueInt64())
	}
	req.ExternalDNSName = in.ExternalDnsName.ValueString()
	req.CertExpiryHrs = ptr.To(int(in.CertExpiryHrs.ValueInt64()))

	req.CalicoNodeCpuLimit = in.CalicoNodeCpuLimit.ValueString()
	req.CalicoNodeMemoryLimit = in.CalicoNodeMemoryLimit.ValueString()
	req.CalicoTyphaCpuLimit = in.CalicoTyphaCpuLimit.ValueString()
	req.CalicoTyphaMemoryLimit = in.CalicoTyphaMemoryLimit.ValueString()
	req.CalicoControllerCpuLimit = in.CalicoControllerCpuLimit.ValueString()
	req.CalicoControllerMemoryLimit = in.CalicoControllerMemoryLimit.ValueString()

	req.DockerPrivateRegistry = in.DockerPrivateRegistry.ValueString()
	req.QuayPrivateRegistry = in.QuayPrivateRegistry.ValueString()
	req.GcrPrivateRegistry = in.GcrPrivateRegistry.ValueString()
	req.K8sPrivateRegistry = in.K8sPrivateRegistry.ValueString()
	req.AddonOperatorImageTag = in.AddonOperatorImageTag.ValueString()

	// TODO: Fix naming violation, whatever in API should be in struct
	req.KubeAPIPort = in.K8sApiPort.ValueString()
	req.DockerRoot = in.DockerRoot.ValueString()

	tagsGoMap := map[string]string{}
	diags = in.Tags.ElementsAs(ctx, &tagsGoMap, false)
	if diags.HasError() {
		return nil, diags
	}
	req.Tags = tagsGoMap
	return &req, diags
}

func getDefaultCreateClusterReq() qbert.CreateClusterRequest {
	return qbert.CreateClusterRequest{
		EtcdBackup: &qbert.EtcdBackupConfig{},
		Monitoring: &qbert.MonitoringConfig{},
	}
}

func (r *clusterResource) attachDetachNodes(ctx context.Context, plan resource_cluster.ClusterModel, state resource_cluster.ClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics
	masterNodesFromPlan := []string{}
	diags.Append(plan.MasterNodes.ElementsAs(ctx, &masterNodesFromPlan, false)...)
	if diags.HasError() {
		return diags
	}
	masterNodesFromState := []string{}
	diags.Append(state.MasterNodes.ElementsAs(ctx, &masterNodesFromState, false)...)
	if diags.HasError() {
		return diags
	}
	diffMasters := findDiff(masterNodesFromState, masterNodesFromPlan)

	workerNodesFromPlan := []string{}
	diags.Append(plan.WorkerNodes.ElementsAs(ctx, &workerNodesFromPlan, false)...)
	if diags.HasError() {
		return diags
	}
	workerNodesFromState := []string{}
	diags.Append(state.WorkerNodes.ElementsAs(ctx, &workerNodesFromState, false)...)
	if diags.HasError() {
		return diags
	}
	diffWorkers := findDiff(workerNodesFromState, workerNodesFromPlan)

	nodeList := []qbert.Node{}
	for _, nodeID := range diffMasters.Removed {
		nodeList = append(nodeList, qbert.Node{
			UUID: nodeID,
		})
	}
	for _, nodeID := range diffWorkers.Removed {
		nodeList = append(nodeList, qbert.Node{
			UUID: nodeID,
		})
	}
	if len(nodeList) > 0 {
		tflog.Debug(ctx, "Detaching nodes", map[string]interface{}{"nodeList": nodeList})
		err := r.client.Qbert().DetachNodes(state.Id.ValueString(), nodeList)
		if err != nil {
			tflog.Error(ctx, "Failed to detach nodes", map[string]interface{}{"error": err})
			diags.AddError("Failed to detach nodes", err.Error())
			return diags
		}
	}
	nodeList = []qbert.Node{}
	for _, nodeID := range diffMasters.Added {
		nodeList = append(nodeList, qbert.Node{
			UUID:     nodeID,
			IsMaster: 1,
		})
	}
	for _, nodeID := range diffWorkers.Added {
		nodeList = append(nodeList, qbert.Node{
			UUID:     nodeID,
			IsMaster: 0,
		})
	}
	if len(nodeList) > 0 {
		tflog.Debug(ctx, "Attaching nodes", map[string]interface{}{"nodeList": nodeList})
		err := r.client.Qbert().AttachNodes(state.Id.ValueString(), nodeList)
		if err != nil {
			// Error: error attaching node <> to cluster <x>: node already attached to cluster <x>
			// TODO: Ignore the above error if the node is already attached to the same cluster
			tflog.Error(ctx, "Failed to attach nodes", map[string]interface{}{"error": err})
			diags.AddError("Failed to attach nodes", err.Error())
			return diags
		}
	}

	return diags
}

type Diff struct {
	Added   []string
	Removed []string
}

func findDiff(slice1, slice2 []string) Diff {
	diff := Diff{}

	// Find added elements
	for _, s := range slice2 {
		found := false
		for _, t := range slice1 {
			if s == t {
				found = true
				break
			}
		}
		if !found {
			diff.Added = append(diff.Added, s)
		}
	}

	// Find removed elements
	for _, s := range slice1 {
		found := false
		for _, t := range slice2 {
			if s == t {
				found = true
				break
			}
		}
		if !found {
			diff.Removed = append(diff.Removed, s)
		}
	}

	return diff
}

// getIntOrNullIfZero returns int64 value if i is not zero, else returns null
// omitempty tag in struct does not work for int64, it returns 0 for null (empty) value
// This is a helper function to convert 0 to null
func getIntOrNullIfZero(i int) basetypes.Int64Value {
	if i == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(i))
}

func getStrOrNullIfEmpty(s string) basetypes.StringValue {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func findLatestKubeRoleVersion(roles []qbert.Role) qbert.Role {
	var latestRole qbert.Role
	// usually the roles are sorted, so the last one is the latest
	latestRole = roles[len(roles)-1]
	for _, role := range roles {
		if role.K8sMajorVersion > latestRole.K8sMajorVersion ||
			(role.K8sMajorVersion == latestRole.K8sMajorVersion && role.K8sMinorVersion > latestRole.K8sMinorVersion) ||
			(role.K8sMajorVersion == latestRole.K8sMajorVersion && role.K8sMinorVersion == latestRole.K8sMinorVersion && role.K8sPatchVersion > latestRole.K8sPatchVersion) ||
			(role.K8sMajorVersion == latestRole.K8sMajorVersion && role.K8sMinorVersion == latestRole.K8sMinorVersion && role.K8sPatchVersion == latestRole.K8sPatchVersion && role.Pf9PatchVersion > latestRole.Pf9PatchVersion) {
			latestRole = role
		}
	}
	return latestRole
}

func areNotMutuallyExclusive(slice1, slice2 []string) bool {
	for _, s := range slice1 {
		for _, t := range slice2 {
			if s == t {
				return true
			}
		}
	}
	return false
}
