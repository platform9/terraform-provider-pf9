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

	"k8s.io/utils/ptr"
)

var _ resource.Resource = (*clusterResource)(nil)
var _ resource.ResourceWithModifyPlan = (*clusterResource)(nil)

func NewClusterResource() resource.Resource {
	return &clusterResource{}
}

type clusterResource struct {
	client       *pmk.HTTPClient
	addonsClient AddonsClient
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
	r.addonsClient = NewAddonClient(r.client.Sunpike())
}

func (r clusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Ref: https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification
	if req.Plan.Raw.IsNull() {
		// resource is being destroyed
		return
	}

	var containersCidr basetypes.StringValue
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("containers_cidr"), &containersCidr)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var servicesCidr basetypes.StringValue
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("services_cidr"), &servicesCidr)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var kubeRoleVersion basetypes.StringValue
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("kube_role_version"), &kubeRoleVersion)...)
	if resp.Diagnostics.HasError() {
		return
	}
	authInfo, err := r.client.Authenticator().Auth(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	if req.State.Raw.IsNull() && !req.Plan.Raw.IsNull() {
		// Pre-Create

		// https://platform9.com/docs/qbert/ref#getprovides-a-list-of-supported-pf9-kube-roles-for-a-cluster-
		supportedKubeRoleVersions, err := r.client.Qbert().ListSupportedVersions(authInfo.ProjectID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to get supported versions", err.Error())
			return
		}
		if !kubeRoleVersion.IsNull() && !kubeRoleVersion.IsUnknown() {
			tflog.Debug(ctx, "Validating if kube_role_version is supported", map[string]interface{}{"kube_role_version": kubeRoleVersion.ValueString()})
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
			if len(supportedKubeRoleVersions.Roles) > 0 {
				latestKubeRoleVersion := findLatestKubeRoleVersion(supportedKubeRoleVersions.Roles)
				resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("kube_role_version"), latestKubeRoleVersion.RoleVersion)...)
				if resp.Diagnostics.HasError() {
					return
				}
			} else {
				resp.Diagnostics.AddError("Failed to get supported versions", "List of supported versions returned by API is empty")
				return
			}
		}

		// Containers & ServiceCidr has different default value for single-node cluster(one-click cluster)
		workerNodes := []string{}
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("worker_nodes"), &workerNodes)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(workerNodes) == 0 {
			if containersCidr.IsNull() || containersCidr.IsUnknown() {
				resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("containers_cidr"), "10.20.0.0/22")...)
			}
			if servicesCidr.IsNull() || servicesCidr.IsUnknown() {
				resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("services_cidr"), "10.21.0.0/22")...)
			}
		} else {
			// UI behavior: If the cluster is a single-node cluster(one-click cluster on UI) then
			// interface_detection_method is not sent to the backend.
			// Otherwise default value of the interface_detection_method is FirstFound
			// This else block corresponds to the case when the cluster is not a single-node cluster.
			var interfaceDetectionMethod basetypes.StringValue
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("interface_detection_method"), &interfaceDetectionMethod)...)
			if resp.Diagnostics.HasError() {
				return
			}
			if interfaceDetectionMethod.IsNull() || interfaceDetectionMethod.IsUnknown() {
				resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("interface_detection_method"), "FirstFound")...)
			}
		}
		var addonsMapVal types.Map
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("addons"), &addonsMapVal)...)
		if resp.Diagnostics.HasError() {
			return
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
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("upgrade_kube_role_version"),
				&upgradeToKubeRoleVersion)...)
			if resp.Diagnostics.HasError() {
				return
			}
			if !upgradeToKubeRoleVersion.IsNull() && !upgradeToKubeRoleVersion.IsUnknown() {
				if !upgradeToKubeRoleVersion.Equal(kubeRoleVersion) {
					resp.Diagnostics.AddAttributeError(path.Root("kube_role_version"), "kube_role_version provided is unsupported",
						fmt.Sprintf("This cluster can only be upgraded to the version: %v", upgradeToKubeRoleVersion.ValueString()))
					return
				}
			} else {
				// This happens when API does not return the next available upgrade version.
				// API returns upgrade versions only when the cluster is in a state to be upgraded.
				// Because of this state does not contain next available upgrade version.
				// TODO: Find workaround, for example call getCluster here and check if it can
				// be upgraded.
				resp.Diagnostics.AddError("Refresh local state", "Cluster is currently being upgraded or local state is out of date")
				return
			}
		}
	}
	if !containersCidr.IsNull() && !servicesCidr.IsNull() {
		isOverlap, err := CheckCIDROverlap(containersCidr.ValueString(), servicesCidr.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error checking cidr overlap", err.Error())
			return
		}
		if isOverlap {
			resp.Diagnostics.AddAttributeError(path.Root("containers_cidr"), "CIDRs overlap", "containers_cidr and services_cidr cannot overlap")
			return
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
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID
	createClusterReq, diags := createCreateClusterRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to create createClusterRequest")
		return
	}
	if !data.NodePoolUuid.IsNull() && !data.NodePoolUuid.IsUnknown() {
		createClusterReq.NodePoolUUID = data.NodePoolUuid.ValueString()
	} else {
		defaultNodePoolUUID, err := r.client.Qbert().GetNodePoolID(projectID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to get default node pool uuid", err.Error())
			return
		}
		tflog.Debug(ctx, "Got default node pool", map[string]interface{}{"nodePoolUUID": defaultNodePoolUUID})
		createClusterReq.NodePoolUUID = defaultNodePoolUUID
	}

	tflog.Info(ctx, "Creating a cluster")
	qbertClient := r.client.Qbert()
	clusterID, err := qbertClient.CreateCluster(createClusterReq, projectID, qbert.CreateClusterOptions{})
	if err != nil {
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
		resp.Diagnostics.AddError("Failed to attach nodes", err.Error())
		return
	}
	// resp.State.SetAttribute(ctx, path.Root("worker_nodes"), data.WorkerNodes)
	// resp.State.SetAttribute(ctx, path.Root("master_nodes"), data.MasterNodes)
	// TODO: Evaluate the feasibility of saving an intermediate state between requests
	// to prevent inconsistency between local and remote state if the provider exits
	// unexpectedly. Consider the overhead, impact on user experience, and alternative
	// approaches to improve reliability.

	if !data.Addons.IsNull() && !data.Addons.IsUnknown() {
		// This is a workaround because default addons are not being set in the plan.
		//
		// Previously, we observed that the default addon parameters were being set correctly
		// until the `ModifyPlan()` function was called. However, after that, the parameter
		// values were no longer being passed to the `Create()` function.
		//
		// Since we cannot set the default addon parameters in the plan due to this issue,
		// we are instead not enabling any addons in the plan. This ensures that the user's
		// intent is respected, even though the backend may still enable the default addons.
		//
		// The "computed_optional" attribute on the "addons" field allows Terraform to
		// successfully apply the plan without raising an error due to the mismatch between
		// the plan and the remote state.
		//
		// This workaround is necessary until the underlying issue with the addon parameters
		// being lost in the `Create()` function is resolved.

		// TODO: Refactor this code to a separate function
		tflog.Debug(ctx, "Getting list of enabled addons")
		defaultEnabledAddons, err := r.listClusterAddons(ctx, clusterID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
			return
		}

		// Create a map key=addonName value=sunpikeAddon for lookup during plan-state comparison
		sunpikeAddonsMap := map[string]sunpikev1alpha2.ClusterAddon{}
		for _, sunpikeAddon := range defaultEnabledAddons {
			sunpikeAddonsMap[sunpikeAddon.Spec.Type] = sunpikeAddon
		}
		tfAddonsMap := map[string]resource_cluster.AddonsValue{}
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("addons"), &tfAddonsMap)...)
		// resp.Diagnostics.Append(data.Addons.ElementsAs(ctx, &tfAddonsMap, false)...)
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
					// If user does not provide the version, we will use the version that is already
					// present in the remote state.
					addonVersion = sunpikeAddon.Spec.Version
				}
				paramsInPlan := map[string]string{}
				resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &paramsInPlan, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
				err := r.addonsClient.Patch(ctx, AddonSpec{
					ClusterID: clusterID,
					Type:      addonName,
					Version:   addonVersion,
					ParamsMap: paramsInPlan,
				}, &sunpikeAddon)
				if err != nil {
					resp.Diagnostics.AddError("Failed to patch addon", err.Error())
					return
				}
			} else {
				// Case 2:
				// The addon in the plan, tfAddon is not present in the remote state, sunpikeAddonsMap.
				// Make the remote state same as the plan state by enabling the addon.
				tflog.Debug(ctx, "Enabling addon", map[string]interface{}{"addon": addonName})
				defaultAddonVersions, err := r.client.Qbert().ListSupportedAddonVersions(ctx, clusterID)
				if err != nil {
					resp.Diagnostics.AddError("Failed to get default addon versions", err.Error())
					return
				}
				paramsInPlan := map[string]string{}
				resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &paramsInPlan, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
				var addonVersion string
				if !tfAddon.Version.IsNull() && !tfAddon.Version.IsUnknown() {
					addonVersion = tfAddon.Version.ValueString()
				} else {
					addonVersion = getDefaultAddonVersion(defaultAddonVersions, addonName)
				}
				if addonVersion == "" {
					resp.Diagnostics.AddError("Failed to get addon version", "Either addon is unknown or version is not provided by the API")
					return
				}
				err = r.addonsClient.Enable(ctx, AddonSpec{
					ClusterID: clusterID,
					Type:      addonName,
					Version:   addonVersion,
					ParamsMap: paramsInPlan,
				})
				if err != nil {
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
				err = r.addonsClient.Disable(ctx, AddonSpec{
					ClusterID: clusterID,
					Type:      addonName,
				})
				if err != nil {
					resp.Diagnostics.AddError("Failed to disable addon", err.Error())
					return
				}
			}
		}
	} // end of addons reconcilation

	// Save data into Terraform state
	resp.Diagnostics.Append(r.readStateFromRemote(ctx, clusterID, projectID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	addonsOnRemote, err := r.listClusterAddons(ctx, clusterID)
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
	addonsOnRemote, err := r.listClusterAddons(ctx, clusterID)
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

	sunpikeAddons, err := r.listClusterAddons(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
		return
	}
	sunpikeAddonsMap := map[string]sunpikev1alpha2.ClusterAddon{}
	for _, sunpikeAddon := range sunpikeAddons {
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
				addonVersion = getDefaultAddonVersion(defaultAddonVersions, addonName)
			} else {
				addonVersion = tfAddon.Version.ValueString()
			}
			paramsInPlan := map[string]string{}
			resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &paramsInPlan, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			err := r.addonsClient.Patch(ctx, AddonSpec{
				ClusterID: clusterID,
				Type:      addonName,
				Version:   addonVersion,
				ParamsMap: paramsInPlan,
			}, &sunpikeAddon)
			if err != nil {
				resp.Diagnostics.AddError("Failed to patch addon", err.Error())
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
				addonVersion = getDefaultAddonVersion(defaultAddonVersions, addonName)
			} else {
				addonVersion = tfAddon.Version.ValueString()
			}
			if addonVersion == "" {
				resp.Diagnostics.AddError("Failed to get addon version", "Either addon is unknown or version is not provided by the API")
				return
			}
			err = r.addonsClient.Enable(ctx, AddonSpec{
				ClusterID: clusterID,
				Type:      addonName,
				Version:   addonVersion,
				ParamsMap: paramsInPlan,
			})
			if err != nil {
				resp.Diagnostics.AddError("Failed to enable addon", err.Error())
				return
			}
		}
	}
	for addonName := range sunpikeAddonsMap {
		if _, found := tfAddonsMap[addonName]; !found {
			tflog.Debug(ctx, "Disabling addon", map[string]interface{}{"addon": addonName})
			err = r.addonsClient.Disable(ctx, AddonSpec{ClusterID: clusterID, Type: addonName})
			if err != nil {
				resp.Diagnostics.AddError("Failed to disable addon", err.Error())
				return
			}
		}
	}

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
		var upgradeClusterReq qbert.UpgradeClusterRequest
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
		// We did not add addonVersions inside upgradeClusterReq;
		// because it will be upgraded using sunpike apis
		err = r.client.Qbert().UpgradeCluster(ctx, upgradeClusterReq, clusterID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to upgrade cluster", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(r.readStateFromRemote(ctx, clusterID, projectID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	sunpikeAddons, err = r.listClusterAddons(ctx, clusterID)
	if err != nil {
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
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}

	projectID := authInfo.ProjectID
	clusterID := data.Id.ValueString()

	tflog.Debug(ctx, "Deleting cluster addons", map[string]interface{}{"clusterID": clusterID})
	err = r.client.Qbert().DeleteAllClusterAddons(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete cluster addons", err.Error())
		return
	}
	tflog.Debug(ctx, "Deleting cluster", map[string]interface{}{"clusterID": clusterID})
	err = r.client.Qbert().DeleteCluster(clusterID, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete cluster", err.Error())
		return
	}
}

func (r *clusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readStateFromRemote sets the values of the attributes in the state variable retrieved from the backend
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

func qbertClusterToTerraformCluster(ctx context.Context, qbertCluster *qbert.Cluster, clusterModel *resource_cluster.ClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics
	clusterModel.Id = types.StringValue(qbertCluster.UUID)
	clusterModel.Name = types.StringValue(qbertCluster.Name)
	clusterModel.AllowWorkloadsOnMaster = types.BoolValue(qbertCluster.AllowWorkloadsOnMaster != 0)
	clusterModel.MasterIp = types.StringValue(qbertCluster.MasterIp)
	clusterModel.MasterVipIface = types.StringValue(qbertCluster.MasterVipIface)
	clusterModel.MasterVipIpv4 = types.StringValue(qbertCluster.MasterVipIpv4)
	clusterModel.ContainersCidr = types.StringValue(qbertCluster.ContainersCidr)
	clusterModel.ServicesCidr = types.StringValue(qbertCluster.ServicesCidr)
	mtuSizeInt, err := strconv.Atoi(qbertCluster.MtuSize)
	if err != nil {
		diags.AddError("Failed to parse mtu size", err.Error())
		return diags
	}
	clusterModel.MtuSize = types.Int64Value(int64(mtuSizeInt))
	clusterModel.Privileged = types.BoolValue(qbertCluster.Privileged != 0)
	clusterModel.UseHostname = types.BoolValue(qbertCluster.UseHostname)
	clusterModel.InterfaceDetectionMethod = types.StringValue(qbertCluster.InterfaceDetectionMethod)
	clusterModel.InterfaceName = types.StringValue(qbertCluster.InterfaceName)
	clusterModel.NodePoolUuid = types.StringValue(qbertCluster.NodePoolUuid)
	// KubeRoleVersion does not change immediately after cluster upgrade
	// hence this is a workaround to get the correct value
	if qbertCluster.UpgradingTo != "" {
		clusterModel.KubeRoleVersion = types.StringValue(qbertCluster.UpgradingTo)
	} else {
		clusterModel.KubeRoleVersion = types.StringValue(qbertCluster.KubeRoleVersion)
	}
	clusterModel.CpuManagerPolicy = types.StringValue(qbertCluster.CPUManagerPolicy)
	clusterModel.TopologyManagerPolicy = types.StringValue(qbertCluster.TopologyManagerPolicy)
	clusterModel.CalicoIpIpMode = types.StringValue(qbertCluster.CalicoIpIpMode)
	clusterModel.CalicoNatOutgoing = types.BoolValue(qbertCluster.CalicoNatOutgoing != 0)
	clusterModel.CalicoV4BlockSize = types.StringValue(qbertCluster.CalicoV4BlockSize)
	clusterModel.CalicoIpv4DetectionMethod = types.StringValue(qbertCluster.CalicoIPv4DetectionMethod)
	clusterModel.NetworkPlugin = types.StringValue(qbertCluster.NetworkPlugin)
	clusterModel.ContainerRuntime = types.StringValue(qbertCluster.ContainerRuntime)
	clusterModel.RuntimeConfig = emptyStringToNull(qbertCluster.RuntimeConfig)

	clusterModel.ExternalDnsName = emptyStringToNull(qbertCluster.ExternalDnsName)
	clusterModel.CertExpiryHrs = types.Int64Value(int64(qbertCluster.CertExpiryHrs))
	clusterModel.CalicoNodeCpuLimit = types.StringValue(qbertCluster.CalicoNodeCpuLimit)
	clusterModel.CalicoNodeMemoryLimit = types.StringValue(qbertCluster.CalicoNodeMemoryLimit)
	clusterModel.CalicoTyphaCpuLimit = types.StringValue(qbertCluster.CalicoTyphaCpuLimit)
	clusterModel.CalicoTyphaMemoryLimit = types.StringValue(qbertCluster.CalicoTyphaMemoryLimit)
	clusterModel.CalicoControllerCpuLimit = types.StringValue(qbertCluster.CalicoControllerCpuLimit)
	clusterModel.CalicoControllerMemoryLimit = types.StringValue(qbertCluster.CalicoControllerMemoryLimit)

	// Computed attributes
	clusterModel.CreatedAt = types.StringValue(qbertCluster.CreatedAt)
	clusterModel.Status = types.StringValue(qbertCluster.Status)
	clusterModel.FlannelIfaceLabel = emptyStringToNull(qbertCluster.FlannelIfaceLabel)
	clusterModel.FlannelPublicIfaceLabel = emptyStringToNull(qbertCluster.FlannelPublicIfaceLabel)
	clusterModel.DockerRoot = types.StringValue(qbertCluster.DockerRoot)
	clusterModel.EtcdDataDir = types.StringValue(qbertCluster.EtcdDataDir)
	clusterModel.LastOp = types.StringValue(qbertCluster.LastOp)
	clusterModel.LastOk = types.StringValue(qbertCluster.LastOk)
	clusterModel.TaskStatus = types.StringValue(qbertCluster.TaskStatus)
	clusterModel.TaskError = types.StringValue(qbertCluster.TaskError)
	clusterModel.ProjectId = types.StringValue(qbertCluster.ProjectId)
	clusterModel.MasterVipVrouterId = types.StringValue(qbertCluster.MasterVipVrouterId)
	clusterModel.K8sApiPort = types.StringValue(qbertCluster.K8sApiPort)
	clusterModel.CalicoIpv4 = types.StringValue(qbertCluster.CalicoIPv4)
	clusterModel.CalicoIpv6 = types.StringValue(qbertCluster.CalicoIPv6)
	clusterModel.CalicoIpv6DetectionMethod = types.StringValue(qbertCluster.CalicoIPv6DetectionMethod)
	clusterModel.CalicoRouterId = types.StringValue(qbertCluster.CalicoRouterID)
	clusterModel.CalicoIpv6PoolCidr = emptyStringToNull(qbertCluster.CalicoIPv6PoolCidr)
	clusterModel.CalicoIpv6PoolBlockSize = types.StringValue(qbertCluster.CalicoIPv6PoolBlockSize)
	clusterModel.CalicoIpv6PoolNatOutgoing = types.BoolValue(qbertCluster.CalicoIPv6PoolNatOutgoing != 0)
	clusterModel.FelixIpv6Support = types.BoolValue(qbertCluster.FelixIPv6Support != 0)
	clusterModel.Masterless = types.BoolValue(qbertCluster.Masterless != 0)
	clusterModel.EtcdVersion = types.StringValue(qbertCluster.EtcdVersion)
	if qbertCluster.EtcdHeartbeatIntervalMs == "" {
		clusterModel.EtcdHeartbeatIntervalMs = types.Int64Null()
	} else {
		etcdHeartbeatIntervalMs, err := strconv.Atoi(qbertCluster.EtcdHeartbeatIntervalMs)
		if err != nil {
			diags.AddError("Failed to parse etcd heartbeat interval", err.Error())
			return diags
		}
		clusterModel.EtcdHeartbeatIntervalMs = types.Int64Value(int64(etcdHeartbeatIntervalMs))
	}
	if qbertCluster.EtcdElectionTimeoutMs == "" {
		clusterModel.EtcdElectionTimeoutMs = types.Int64Null()
	} else {
		etcdElectionTimeoutMs, err := strconv.Atoi(qbertCluster.EtcdElectionTimeoutMs)
		if err != nil {
			diags.AddError("Failed to parse etcd election timeout", err.Error())
			return diags
		}
		clusterModel.EtcdElectionTimeoutMs = types.Int64Value(int64(etcdElectionTimeoutMs))
	}
	clusterModel.MasterStatus = types.StringValue(qbertCluster.MasterStatus)
	clusterModel.WorkerStatus = types.StringValue(qbertCluster.WorkerStatus)
	clusterModel.Ipv6 = types.BoolValue(qbertCluster.IPv6 != 0)
	clusterModel.NodePoolName = types.StringValue(qbertCluster.NodePoolName)
	clusterModel.CloudProviderUuid = types.StringValue(qbertCluster.CloudProviderUuid)
	clusterModel.CloudProviderName = types.StringValue(qbertCluster.CloudProviderName)
	clusterModel.CloudProviderType = types.StringValue(qbertCluster.CloudProviderType)
	clusterModel.DockerPrivateRegistry = types.StringValue(qbertCluster.DockerPrivateRegistry)
	clusterModel.QuayPrivateRegistry = types.StringValue(qbertCluster.QuayPrivateRegistry)
	clusterModel.GcrPrivateRegistry = types.StringValue(qbertCluster.GcrPrivateRegistry)
	clusterModel.K8sPrivateRegistry = types.StringValue(qbertCluster.K8sPrivateRegistry)
	clusterModel.DockerCentosPackageRepoUrl = types.StringValue(qbertCluster.DockerCentosPackageRepoUrl)
	clusterModel.DockerUbuntuPackageRepoUrl = types.StringValue(qbertCluster.DockerUbuntuPackageRepoUrl)
	clusterModel.InterfaceReachableIp = types.StringValue(qbertCluster.InterfaceReachableIP)
	clusterModel.CustomRegistryUrl = types.StringValue(qbertCluster.CustomRegistryUrl)
	clusterModel.CustomRegistryRepoPath = types.StringValue(qbertCluster.CustomRegistryRepoPath)
	clusterModel.CustomRegistryUsername = types.StringValue(qbertCluster.CustomRegistryUsername)
	clusterModel.CustomRegistryPassword = types.StringValue(qbertCluster.CustomRegistryPassword)
	clusterModel.CustomRegistrySkipTls = types.BoolValue(qbertCluster.CustomRegistrySkipTls != 0)
	clusterModel.CustomRegistrySelfSignedCerts = types.BoolValue(qbertCluster.CustomRegistrySelfSignedCerts != 0)
	clusterModel.CustomRegistryCertPath = types.StringValue(qbertCluster.CustomRegistryCertPath)
	if qbertCluster.CanUpgrade {
		if qbertCluster.CanMinorUpgrade == 1 {
			clusterModel.UpgradeKubeRoleVersion = types.StringValue(qbertCluster.MinorUpgradeRoleVersion)
		} else if qbertCluster.CanPatchUpgrade == 1 {
			clusterModel.UpgradeKubeRoleVersion = types.StringValue(qbertCluster.PatchUpgradeRoleVersion)
		} else {
			clusterModel.UpgradeKubeRoleVersion = types.StringNull()
		}
	} else {
		clusterModel.UpgradeKubeRoleVersion = types.StringNull()
	}

	if qbertCluster.EnableEtcdEncryption == "true" {
		clusterModel.EnableEtcdEncryption = types.BoolValue(true)
	} else {
		clusterModel.EnableEtcdEncryption = types.BoolValue(false)
	}
	if qbertCluster.EtcdBackup != nil {
		var localPathVal types.String
		storageProps := qbertCluster.EtcdBackup.StorageProperties
		if storageProps.LocalPath != nil {
			localPathVal = types.StringValue(*qbertCluster.EtcdBackup.StorageProperties.LocalPath)
		} else {
			localPathVal = types.StringNull()
		}
		etcdBackupObjVal, convertDiags := resource_cluster.EtcdBackupValue{
			IsEtcdBackupEnabled:     types.BoolValue(qbertCluster.EtcdBackup.IsEtcdBackupEnabled != 0),
			StorageType:             types.StringValue(qbertCluster.EtcdBackup.StorageType),
			MaxTimestampBackupCount: getIntOrNullIfZero(qbertCluster.EtcdBackup.MaxTimestampBackupCount),
			StorageLocalPath:        localPathVal,
			DailyBackupTime:         getStrOrNullIfEmpty(qbertCluster.EtcdBackup.DailyBackupTime),
			IntervalInHours:         getIntOrNullIfZero(qbertCluster.EtcdBackup.IntervalInHours),
			IntervalInMins:          getIntOrNullIfZero(qbertCluster.EtcdBackup.IntervalInMins),
			MaxIntervalBackupCount:  getIntOrNullIfZero(qbertCluster.EtcdBackup.MaxIntervalBackupCount),
		}.ToObjectValue(ctx)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return diags
		}
		etcdBackup, convertDiags := resource_cluster.NewEtcdBackupValue(
			etcdBackupObjVal.AttributeTypes(ctx), etcdBackupObjVal.Attributes())
		diags.Append(convertDiags...)
		if diags.HasError() {
			return diags
		}
		clusterModel.EtcdBackup = etcdBackup
	}
	if len(qbertCluster.Tags) == 0 {
		clusterModel.Tags = types.MapNull(basetypes.StringType{})
	} else {
		tagsGoMap := map[string]attr.Value{}
		for key, val := range qbertCluster.Tags {
			tagsGoMap[key] = types.StringValue(val)
		}
		tfMap, convertDiags := types.MapValueFrom(ctx, types.StringType, tagsGoMap)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return diags
		}
		clusterModel.Tags = tfMap
	}

	return diags
}

func createCreateClusterRequest(ctx context.Context, clusterModel *resource_cluster.ClusterModel) (qbert.CreateClusterRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	createClusterReq := qbert.CreateClusterRequest{
		EtcdBackup: &qbert.EtcdBackupConfig{},
		Monitoring: &qbert.MonitoringConfig{},
	}
	createClusterReq.Name = clusterModel.Name.ValueString()
	createClusterReq.Privileged = clusterModel.Privileged.ValueBoolPointer()
	createClusterReq.MasterIP = clusterModel.MasterIp.ValueString()
	masterNodes := []string{}
	diags.Append(clusterModel.MasterNodes.ElementsAs(ctx, &masterNodes, false)...)
	if diags.HasError() {
		return createClusterReq, diags
	}
	createClusterReq.MasterNodes = masterNodes

	if !clusterModel.WorkerNodes.IsNull() && !clusterModel.WorkerNodes.IsUnknown() {
		workerNodes := []string{}
		diags.Append(clusterModel.WorkerNodes.ElementsAs(ctx, &workerNodes, false)...)
		if diags.HasError() {
			return createClusterReq, diags
		}
		createClusterReq.WorkerNodes = workerNodes
		if areNotMutuallyExclusive(masterNodes, workerNodes) {
			diags.AddAttributeError(path.Root("worker_nodes"), "worker_nodes and master_nodes should be mutually exclusive", "Same node can not be part of both worker and master nodes")
			return createClusterReq, diags
		}
	}
	createClusterReq.AllowWorkloadOnMaster = clusterModel.AllowWorkloadsOnMaster.ValueBoolPointer()
	createClusterReq.MasterVirtualIPIface = clusterModel.MasterVipIface.ValueString()
	createClusterReq.MasterVirtualIP = clusterModel.MasterVipIpv4.ValueString()
	createClusterReq.ContainerCIDR = clusterModel.ContainersCidr.ValueString()
	createClusterReq.ServiceCIDR = clusterModel.ServicesCidr.ValueString()
	createClusterReq.MTUSize = ptr.To(int(clusterModel.MtuSize.ValueInt64()))
	createClusterReq.Privileged = clusterModel.Privileged.ValueBoolPointer()
	createClusterReq.UseHostname = clusterModel.UseHostname.ValueBoolPointer()
	createClusterReq.InterfaceDetectionMethod = clusterModel.InterfaceDetectionMethod.ValueString()
	createClusterReq.InterfaceName = clusterModel.InterfaceName.ValueString()
	createClusterReq.KubeRoleVersion = clusterModel.KubeRoleVersion.ValueString()
	createClusterReq.CPUManagerPolicy = clusterModel.CpuManagerPolicy.ValueString()
	createClusterReq.ExternalDNSName = clusterModel.ExternalDnsName.ValueString()
	createClusterReq.TopologyManagerPolicy = clusterModel.TopologyManagerPolicy.ValueString()
	createClusterReq.CalicoIPIPMode = clusterModel.CalicoIpIpMode.ValueString()
	createClusterReq.CalicoNatOutgoing = clusterModel.CalicoNatOutgoing.ValueBoolPointer()
	createClusterReq.CalicoV4BlockSize = clusterModel.CalicoV4BlockSize.ValueString()
	createClusterReq.CalicoIpv4DetectionMethod = clusterModel.CalicoIpv4DetectionMethod.ValueString()
	createClusterReq.NetworkPlugin = qbert.CNIBackend(clusterModel.NetworkPlugin.ValueString())
	createClusterReq.RuntimeConfig = clusterModel.RuntimeConfig.ValueString()
	createClusterReq.ContainerRuntime = qbert.ContainerRuntime(clusterModel.ContainerRuntime.ValueString())

	if !clusterModel.EnableEtcdEncryption.IsUnknown() && !clusterModel.EnableEtcdEncryption.IsNull() {
		createClusterReq.EnableEtcdEncryption = fmt.Sprintf("%v", clusterModel.EnableEtcdEncryption.ValueBool())
	}
	createClusterReq.EtcdBackup.DailyBackupTime = clusterModel.EtcdBackup.DailyBackupTime.ValueString()
	if clusterModel.EtcdBackup.IsEtcdBackupEnabled.ValueBool() {
		createClusterReq.EtcdBackup.IsEtcdBackupEnabled = 1
	} else {
		createClusterReq.EtcdBackup.IsEtcdBackupEnabled = 0
	}
	createClusterReq.EtcdBackup.MaxTimestampBackupCount = int(clusterModel.EtcdBackup.MaxTimestampBackupCount.ValueInt64())
	createClusterReq.EtcdBackup.StorageProperties.LocalPath = clusterModel.EtcdBackup.StorageLocalPath.ValueStringPointer()
	createClusterReq.EtcdBackup.StorageType = clusterModel.EtcdBackup.StorageType.ValueString()
	createClusterReq.EtcdBackup.IntervalInHours = int(clusterModel.EtcdBackup.IntervalInHours.ValueInt64())
	createClusterReq.EtcdBackup.IntervalInMins = int(clusterModel.EtcdBackup.IntervalInMins.ValueInt64())
	if !clusterModel.EtcdBackup.MaxIntervalBackupCount.IsUnknown() && !clusterModel.EtcdBackup.MaxIntervalBackupCount.IsNull() {
		createClusterReq.EtcdBackup.MaxIntervalBackupCount = int(clusterModel.EtcdBackup.MaxIntervalBackupCount.ValueInt64())
	}
	createClusterReq.ExternalDNSName = clusterModel.ExternalDnsName.ValueString()
	if !clusterModel.CertExpiryHrs.IsNull() && !clusterModel.CertExpiryHrs.IsUnknown() {
		createClusterReq.CertExpiryHrs = ptr.To(int(clusterModel.CertExpiryHrs.ValueInt64()))
	}
	createClusterReq.CalicoNodeCpuLimit = clusterModel.CalicoNodeCpuLimit.ValueString()
	createClusterReq.CalicoNodeMemoryLimit = clusterModel.CalicoNodeMemoryLimit.ValueString()
	createClusterReq.CalicoTyphaCpuLimit = clusterModel.CalicoTyphaCpuLimit.ValueString()
	createClusterReq.CalicoTyphaMemoryLimit = clusterModel.CalicoTyphaMemoryLimit.ValueString()
	createClusterReq.CalicoControllerCpuLimit = clusterModel.CalicoControllerCpuLimit.ValueString()
	createClusterReq.CalicoControllerMemoryLimit = clusterModel.CalicoControllerMemoryLimit.ValueString()

	createClusterReq.DockerPrivateRegistry = clusterModel.DockerPrivateRegistry.ValueString()
	createClusterReq.QuayPrivateRegistry = clusterModel.QuayPrivateRegistry.ValueString()
	createClusterReq.GcrPrivateRegistry = clusterModel.GcrPrivateRegistry.ValueString()
	createClusterReq.K8sPrivateRegistry = clusterModel.K8sPrivateRegistry.ValueString()

	createClusterReq.KubeAPIPort = clusterModel.K8sApiPort.ValueString()
	createClusterReq.DockerRoot = clusterModel.DockerRoot.ValueString()

	tagsGoMap := map[string]string{}
	diags = clusterModel.Tags.ElementsAs(ctx, &tagsGoMap, false)
	if diags.HasError() {
		return createClusterReq, diags
	}
	createClusterReq.Tags = tagsGoMap
	return createClusterReq, diags
}

func (r *clusterResource) listClusterAddons(ctx context.Context, clusterID string) ([]sunpikev1alpha2.ClusterAddon, error) {
	tflog.Info(ctx, "Listing addons enabled on the cluster", map[string]interface{}{"clusterID": clusterID})
	sunpikeAddonsList, err := r.client.Qbert().ListClusterAddons(fmt.Sprintf("sunpike.pf9.io/cluster=%s", clusterID))
	if err != nil {
		return nil, err
	}
	return sunpikeAddonsList.Items, nil
}
