package provider

import (
	"context"
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
var _ resource.ResourceWithModifyPlan = (*clusterResource)(nil)

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

func (r clusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Ref: https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification
	if req.Plan.Raw.IsNull() {
		// resource is being destroyed
		return
	}

	var kubeRoleVersion basetypes.StringValue
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("kube_role_version"), &kubeRoleVersion)...)
	if resp.Diagnostics.HasError() {
		return
	}
	authInfo, err := r.client.Authenticator().Auth(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to authenticate", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	if req.State.Raw.IsNull() && !req.Plan.Raw.IsNull() {
		// Pre-Create
		if !kubeRoleVersion.IsNull() && !kubeRoleVersion.IsUnknown() {
			tflog.Debug(ctx, "Validating if kube_role_version is supported", map[string]interface{}{"kube_role_version": kubeRoleVersion.ValueString()})
			supportedKubeRoleVersions, err := r.client.Qbert().ListSupportedVersions(authInfo.ProjectID)
			if err != nil {
				resp.Diagnostics.AddError("Failed to get supported versions", err.Error())
				return
			}
			allowedKubeRoleVersions := []string{}
			for _, role := range supportedKubeRoleVersions.Roles {
				allowedKubeRoleVersions = append(allowedKubeRoleVersions, role.RoleVersion)
			}
			if !StrSliceContains(allowedKubeRoleVersions, kubeRoleVersion.ValueString()) {
				resp.Diagnostics.AddAttributeError(path.Root("kube_role_version"), "kube_role_version provided is unsupported", fmt.Sprintf("Supported versions: %v", allowedKubeRoleVersions))
				return
			}
		} else {
			tflog.Debug(ctx, "kube_role_version is not provided in the plan; defaulting to the latest")
			// https://platform9.com/docs/qbert/ref#getprovides-a-list-of-supported-pf9-kube-roles-for-a-cluster-
			supportedKubeRoleVersions, err := r.client.Qbert().ListSupportedVersions(authInfo.ProjectID)
			if err != nil {
				tflog.Error(ctx, "Failed to get supported versions", map[string]interface{}{"error": err})
				resp.Diagnostics.AddError("Failed to get supported versions", err.Error())
				return
			}
			if len(supportedKubeRoleVersions.Roles) > 0 {
				latestKubeRoleVersion := findLatestKubeRoleVersion(supportedKubeRoleVersions.Roles)
				resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("kube_role_version"), latestKubeRoleVersion.RoleVersion)...)
				if resp.Diagnostics.HasError() {
					return
				}
			} else {
				resp.Diagnostics.AddError("No supported kube role versions found", "No supported kube role versions found")
				return
			}
		}
	}
	if !req.State.Raw.IsNull() && !req.Plan.Raw.IsNull() {
		// Pre-Update
		var stateKubeRoleVersion basetypes.StringValue
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("kube_role_version"),
			&stateKubeRoleVersion)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !kubeRoleVersion.Equal(stateKubeRoleVersion) {
			var upgradeToKubeRoleVersion basetypes.StringValue
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("upgrade_kube_role_version"),
				&upgradeToKubeRoleVersion)...)
			if resp.Diagnostics.HasError() {
				return
			}
			if !upgradeToKubeRoleVersion.Equal(kubeRoleVersion) {
				resp.Diagnostics.AddAttributeError(path.Root("kube_role_version"), "kube_role_version provided is unsupported",
					fmt.Sprintf("This cluster can only be upgraded to the version: %v", upgradeToKubeRoleVersion.ValueString()))
				return
			}
		}
	}
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

	createClusterReq, d := r.CreateCreateClusterRequest(ctx, authInfo.ProjectID, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to create create cluster object")
		return
	}

	tflog.Info(ctx, "Creating a cluster")
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
	// This will prevent inconsistency between the local state and the remote state
	tflog.Debug(ctx, "Getting list of enabled addons")
	sunpikeAddons, err := r.readAddonsFromRemote(ctx, clusterID)
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster addons", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
		return
	}

	// Create a map key=addonName value=sunpikeAddon for lookup during plan-state comparison
	sunpikeAddonsMap := map[string]sunpikev1alpha2.ClusterAddon{}
	for _, sunpikeAddon := range sunpikeAddons {
		sunpikeAddonsMap[sunpikeAddon.Spec.Type] = sunpikeAddon
	}
	defaultAddonVersions, err := r.client.Qbert().ListSupportedAddonVersions(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get default addon versions", err.Error())
		return
	}
	tfAddonsMap := map[string]resource_cluster.AddonsValue{}
	resp.Diagnostics.Append(data.Addons.ElementsAs(ctx, &tfAddonsMap, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for addonName, tfAddon := range tfAddonsMap {
		// sunpikeAddon represents remote state and tfAddon represents plan state
		if sunpikeAddon, found := sunpikeAddonsMap[addonName]; found {
			// Case 1:
			// if addon with the same name is available at both places, difference bw
			// the two should be patched, prefering the plan instance.
			tflog.Debug(ctx, "Checking if addon version and params needs to be patched")
			var addonVersion string
			if !tfAddon.Version.IsNull() && !tfAddon.Version.IsUnknown() {
				addonVersion = tfAddon.Version.ValueString()
			} else {
				// version is optional in the plan, because user cannot determine the version.
				// API call provides the default version
				addonVersion = defaultAddonVersions[addonName]
			}
			paramsInPlan := map[string]string{}
			resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &paramsInPlan, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			resp.Diagnostics.Append(r.patchAddon(ctx, &sunpikeAddon, AddonSpec{
				ClusterID: clusterID,
				Type:      addonName,
				Version:   addonVersion,
				ParamsMap: paramsInPlan,
			})...)
			if resp.Diagnostics.HasError() {
				return
			}
		} else {
			// Case 2:
			// The addon in the plan, tfAddon is not present in the remote state, sunpikeAddonsMap.
			// Make the remote state same as the plan state by enabling the addon.
			tflog.Debug(ctx, "Enabling addon", map[string]interface{}{"addon": addonName})
			paramsInPlan := map[string]string{}
			resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &paramsInPlan, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			var addonVersion string
			if !tfAddon.Version.IsNull() && !tfAddon.Version.IsUnknown() {
				addonVersion = tfAddon.Version.ValueString()
			} else {
				addonVersion = defaultAddonVersions[addonName]
			}
			err = r.enableAddon(ctx, AddonSpec{
				ClusterID: clusterID,
				Type:      addonName,
				Version:   addonVersion,
				ParamsMap: paramsInPlan,
			})
			if err != nil {
				tflog.Error(ctx, "Failed to enable addon", map[string]interface{}{"addon": addonName, "error": err})
				resp.Diagnostics.AddError("Failed to enable addon", err.Error())
				return
			}
		}
	}
	for addonName := range sunpikeAddonsMap {
		if _, found := tfAddonsMap[addonName]; !found {
			// Case 3:
			// The addon is present in the remote state, sunpikeAddonsMap
			// but not present in the plan, tfAddonsMap. Disabling the addon
			// will make the remote state same as the plan state.
			tflog.Debug(ctx, "Disabling addon", map[string]interface{}{"addon": addonName})
			err = r.disableAddon(ctx, clusterID, addonName)
			if err != nil {
				tflog.Error(ctx, "Failed to disable addon", map[string]interface{}{"addon": addonName, "error": err})
				resp.Diagnostics.AddError("Failed to disable addon", err.Error())
				return
			}
		}
	}

	resp.Diagnostics.Append(r.readStateFromRemote(ctx, clusterID, projectID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	addonsOnRemote, err := r.readAddonsFromRemote(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
		return
	}
	tfAddonsMapState, diags := sunpikeAddonsToTerraformAddons(ctx, addonsOnRemote)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.Addons, diags = types.MapValueFrom(ctx, resource_cluster.AddonsValue{}.Type(ctx), tfAddonsMapState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// This attr is useful in Update only, copied value from state to prevent inconsistency
	state.BatchUpgradePercent = data.BatchUpgradePercent
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func sunpikeAddonsToTerraformAddons(ctx context.Context, sunpikeAddons []sunpikev1alpha2.ClusterAddon) (map[string]resource_cluster.AddonsValue, diag.Diagnostics) {
	tfAddonsMap := map[string]resource_cluster.AddonsValue{}
	var diags diag.Diagnostics
	for _, sunpikeAddon := range sunpikeAddons {
		version := types.StringValue(sunpikeAddon.Spec.Version)
		phase := types.StringValue(string(sunpikeAddon.Status.Phase))
		paramMap := map[string]string{}
		for _, param := range sunpikeAddon.Spec.Override.Params {
			paramMap[param.Name] = param.Value
		}
		var params basetypes.MapValue
		params, diags = types.MapValueFrom(ctx, types.StringType, paramMap)
		if diags.HasError() {
			return tfAddonsMap, diags
		}
		addonObjVal, diags := resource_cluster.AddonsValue{
			Version: version,
			Phase:   phase,
			Params:  params,
		}.ToObjectValue(ctx)
		if diags.HasError() {
			return tfAddonsMap, diags
		}
		addonObjValuable, diags := resource_cluster.AddonsType{}.ValueFromObject(ctx, addonObjVal)
		if diags.HasError() {
			return tfAddonsMap, diags
		}
		tfAddonsMap[sunpikeAddon.Spec.Type] = addonObjValuable.(resource_cluster.AddonsValue)
	}
	return tfAddonsMap, diags
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
	addonsOnRemote, err := r.readAddonsFromRemote(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get sunpike addons", err.Error())
		return
	}
	tfAddonsMapState, diags := sunpikeAddonsToTerraformAddons(ctx, addonsOnRemote)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.Addons, diags = types.MapValueFrom(ctx, resource_cluster.AddonsValue{}.Type(ctx), tfAddonsMapState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	// Check if update works for this fields
	if !plan.CertExpiryHrs.Equal(state.CertExpiryHrs) {
		editRequired = true
		editClusterReq.CertExpiryHrs = int(plan.CertExpiryHrs.ValueInt64())
	}

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
		tflog.Info(ctx, "Requested upgrade of the cluster", map[string]interface{}{"from": state.KubeRoleVersion, "to": plan.KubeRoleVersion})
		tflog.Info(ctx, "Reading cluster from qbert", map[string]interface{}{"clusterID": clusterID})
		cluster, err := r.client.Qbert().GetCluster(ctx, projectID, clusterID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to get cluster", err.Error())
			return
		}
		if !cluster.CanUpgrade {
			resp.Diagnostics.AddError("Cluster cannot be upgraded", "Cluster is not in a state to be upgraded")
			return
		}
		var allowedTargetVersion string
		if cluster.CanMinorUpgrade == 1 {
			allowedTargetVersion = cluster.MinorUpgradeRoleVersion
			upgradeClusterReq.KubeRoleVersionUpgradeType = "minor"
		}
		if cluster.CanPatchUpgrade == 1 {
			allowedTargetVersion = cluster.PatchUpgradeRoleVersion
			upgradeClusterReq.KubeRoleVersionUpgradeType = "patch"
		}
		if allowedTargetVersion == "" {
			resp.Diagnostics.AddError("Cluster cannot be upgraded", "Cluster is not in a state to be upgraded")
			return
		}
		planVersion := plan.KubeRoleVersion.ValueString()
		if planVersion != allowedTargetVersion {
			resp.Diagnostics.AddError("Kube role version cannot be upgraded to this version", fmt.Sprintf("Allowed version is: %v", allowedTargetVersion))
			return
		}
		if !plan.BatchUpgradePercent.IsNull() && !plan.BatchUpgradePercent.IsUnknown() {
			upgradeClusterReq.BatchUpgradePercent = int(plan.BatchUpgradePercent.ValueInt64())
		}
		upgradeRequired = true
	}

	// We did not add addonVersions inside upgradeClusterReq; because it will be upgraded using sunpike apis later
	if upgradeRequired {
		err = r.client.Qbert().UpgradeCluster(ctx, upgradeClusterReq, clusterID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to upgrade cluster", err.Error())
			return
		}
	}

	tflog.Debug(ctx, "Getting list of enabled addons")
	sunpikeAddonsList, err := r.client.Qbert().ListClusterAddons(fmt.Sprintf("sunpike.pf9.io/cluster=%s", clusterID))
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster addons", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
		return
	}
	sunpikeAddonsMap := map[string]sunpikev1alpha2.ClusterAddon{}
	for _, sunpikeAddon := range sunpikeAddonsList.Items {
		sunpikeAddonsMap[sunpikeAddon.Spec.Type] = sunpikeAddon
	}
	// Load plan addons into a map
	tfAddonsMap := map[string]resource_cluster.AddonsValue{}
	resp.Diagnostics.Append(plan.Addons.ElementsAs(ctx, &tfAddonsMap, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defaultAddonVersions, err := r.client.Qbert().ListSupportedAddonVersions(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get default addon versions", err.Error())
		return
	}
	for addonName, tfAddon := range tfAddonsMap {
		if sunpikeAddon, found := sunpikeAddonsMap[addonName]; found {
			// Patch the addon
			tflog.Debug(ctx, "Checking if addon version and params needs to be patched")
			var addonVersion string
			if tfAddon.Version.IsNull() || tfAddon.Version.IsUnknown() {
				tflog.Debug(ctx, "Version is not provided in the plan, getting default version")
				addonVersion = defaultAddonVersions[addonName]
			} else {
				addonVersion = tfAddon.Version.ValueString()
			}
			paramsInPlan := map[string]string{}
			resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &paramsInPlan, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			resp.Diagnostics.Append(r.patchAddon(ctx, &sunpikeAddon, AddonSpec{
				ClusterID: clusterID,
				Type:      addonName,
				Version:   addonVersion,
				ParamsMap: paramsInPlan,
			})...)
			if resp.Diagnostics.HasError() {
				return
			}
		} else {
			// Enable the addon
			tflog.Debug(ctx, "Enabling addon", map[string]interface{}{"addon": addonName})
			paramsInPlan := map[string]string{}
			resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &paramsInPlan, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			var addonVersion string
			if tfAddon.Version.IsNull() || tfAddon.Version.IsUnknown() {
				tflog.Debug(ctx, "Version is not provided in the plan, getting default version")
				addonVersion = defaultAddonVersions[addonName]
			} else {
				addonVersion = tfAddon.Version.ValueString()
			}
			err = r.enableAddon(ctx, AddonSpec{
				ClusterID: clusterID,
				Type:      addonName,
				Version:   addonVersion,
				ParamsMap: paramsInPlan,
			})
			if err != nil {
				tflog.Error(ctx, "Failed to enable addon", map[string]interface{}{"addon": addonName, "error": err})
				resp.Diagnostics.AddError("Failed to enable addon", err.Error())
				return
			}
		}
	}
	for addonName := range sunpikeAddonsMap {
		if _, found := tfAddonsMap[addonName]; !found {
			tflog.Debug(ctx, "Disabling addon", map[string]interface{}{"addon": addonName})
			err = r.disableAddon(ctx, clusterID, addonName)
			if err != nil {
				tflog.Error(ctx, "Failed to disable addon", map[string]interface{}{"addon": addonName, "error": err})
				resp.Diagnostics.AddError("Failed to disable addon", err.Error())
				return
			}
		}
	}

	resp.Diagnostics.Append(r.readStateFromRemote(ctx, clusterID, projectID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	sunpikeAddons, err := r.readAddonsFromRemote(ctx, clusterID)
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster addons", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
		return
	}
	tfAddonsMapState, diags := sunpikeAddonsToTerraformAddons(ctx, sunpikeAddons)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.Addons, diags = types.MapValueFrom(ctx, resource_cluster.AddonsValue{}.Type(ctx), tfAddonsMapState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the batch upgrade percent from the plan to the state
	// To prevent inconsistency. This attr is read only in case
	// of upgrade cluster, it is not associated with any remote attribute
	state.BatchUpgradePercent = plan.BatchUpgradePercent
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
	out.RuntimeConfig = emptyStringToNull(in.RuntimeConfig)

	out.ExternalDnsName = emptyStringToNull(in.ExternalDnsName)
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
	out.FlannelIfaceLabel = emptyStringToNull(in.FlannelIfaceLabel)
	out.FlannelPublicIfaceLabel = emptyStringToNull(in.FlannelPublicIfaceLabel)
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
	out.CalicoIpv6PoolCidr = emptyStringToNull(in.CalicoIPv6PoolCidr)
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
	out.InterfaceReachableIp = types.StringValue(in.InterfaceReachableIP)
	out.CustomRegistryUrl = types.StringValue(in.CustomRegistryUrl)
	out.CustomRegistryRepoPath = types.StringValue(in.CustomRegistryRepoPath)
	out.CustomRegistryUsername = types.StringValue(in.CustomRegistryUsername)
	out.CustomRegistryPassword = types.StringValue(in.CustomRegistryPassword)
	out.CustomRegistrySkipTls = types.BoolValue(in.CustomRegistrySkipTls != 0)
	out.CustomRegistrySelfSignedCerts = types.BoolValue(in.CustomRegistrySelfSignedCerts != 0)
	out.CustomRegistryCertPath = types.StringValue(in.CustomRegistryCertPath)
	if in.CanUpgrade {
		if in.CanMinorUpgrade == 1 {
			out.UpgradeKubeRoleVersion = types.StringValue(in.MinorUpgradeRoleVersion)
		} else if in.CanPatchUpgrade == 1 {
			out.UpgradeKubeRoleVersion = types.StringValue(in.PatchUpgradeRoleVersion)
		} else {
			out.UpgradeKubeRoleVersion = types.StringNull()
		}
	} else {
		out.UpgradeKubeRoleVersion = types.StringNull()
	}

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
		// TODO: Use value.ToObjectValue() and then type.ValueFromObject() instead of creating the object manually
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
	if !in.CertExpiryHrs.IsNull() && !in.CertExpiryHrs.IsUnknown() {
		req.CertExpiryHrs = ptr.To(int(in.CertExpiryHrs.ValueInt64()))
	}
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

// qbert API returns empty string for null values, this function converts empty string to null to prevent
// Provider produced inconsistent result after apply, .external_dns_name: was null, but now cty.StringVal("")
func emptyStringToNull(s string) basetypes.StringValue {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
