package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/resource_cluster"

	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"

	"k8s.io/utils/ptr"
)

var _ resource.Resource = (*clusterResource)(nil)
var _ resource.ResourceWithModifyPlan = (*clusterResource)(nil)
var _ resource.ResourceWithValidateConfig = (*clusterResource)(nil)

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

func (c clusterResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data resource_cluster.ClusterModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workerNodes := []types.String{}
	if !data.WorkerNodes.IsNull() && !data.WorkerNodes.IsUnknown() {
		resp.Diagnostics.Append(data.WorkerNodes.ElementsAs(ctx, &workerNodes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	masterNodes := []types.String{}
	if !data.MasterNodes.IsNull() && !data.MasterNodes.IsUnknown() {
		resp.Diagnostics.Append(data.MasterNodes.ElementsAs(ctx, &masterNodes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	for _, m := range masterNodes {
		for _, w := range workerNodes {
			if m == w {
				resp.Diagnostics.AddAttributeError(path.Root("worker_nodes"), "Master and worker nodes overlap",
					fmt.Sprintf("The node with ID %v is configured to be part of both the master and worker nodes, which is not allowed."+
						" Each node must be assigned to either the master or the worker role, but not both.", m))
				return
			}
		}
	}
	if len(workerNodes) == 0 && !data.WorkerNodes.IsUnknown() {
		var allowWorkloadsOnMaster types.Bool
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("allow_workloads_on_master"), &allowWorkloadsOnMaster)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !allowWorkloadsOnMaster.IsNull() && !allowWorkloadsOnMaster.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root("worker_nodes"), "worker_nodes is required", "The allow_workloads_on_master should be true or worker_nodes should be provided")
			return
		}
	}

	if !data.ContainersCidr.IsNull() && !data.ContainersCidr.IsUnknown() &&
		!data.ServicesCidr.IsNull() && !data.ServicesCidr.IsUnknown() {
		isOverlap, err := CheckCIDROverlap(data.ContainersCidr.ValueString(), data.ServicesCidr.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error checking cidr overlap", err.Error())
			return
		}
		if isOverlap {
			resp.Diagnostics.AddAttributeError(path.Root("containers_cidr"), "CIDRs overlap", "containers_cidr and services_cidr cannot overlap")
		}
	}
}

func (r clusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Ref: https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification
	if req.Plan.Raw.IsNull() {
		// resource is being destroyed
		return
	}

	var kubeRoleVersion types.String
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
				resp.Diagnostics.AddAttributeError(path.Root("kube_role_version"), "Provided value is not supported", fmt.Sprintf("Supported versions: %v", allowedKubeRoleVersions))
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

		var workerNodesSet types.Set
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("worker_nodes"), &workerNodesSet)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !workerNodesSet.IsNull() && !workerNodesSet.IsUnknown() {
			workerNodes := []types.String{}
			resp.Diagnostics.Append(workerNodesSet.ElementsAs(ctx, &workerNodes, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			strWorkerNodes := []string{}
			for _, nodeID := range workerNodes {
				if !nodeID.IsNull() && !nodeID.IsUnknown() {
					strWorkerNodes = append(strWorkerNodes, nodeID.ValueString())
				}
			}
			if len(strWorkerNodes) == 0 {
				var containersCidr types.String
				resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("containers_cidr"), &containersCidr)...)
				if resp.Diagnostics.HasError() {
					return
				}
				var servicesCidr types.String
				resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("services_cidr"), &servicesCidr)...)
				if resp.Diagnostics.HasError() {
					return
				}
				// Containers & ServiceCidr has different default value for single-node cluster(worker_nodes=0)
				if containersCidr.IsNull() {
					resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("containers_cidr"), "10.20.0.0/22")...)
				}
				if servicesCidr.IsNull() {
					resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("services_cidr"), "10.21.0.0/22")...)
				}
			}
		}
	}
	if !req.State.Raw.IsNull() && !req.Plan.Raw.IsNull() {
		// Pre-Update
		var stateKubeRoleVersion types.String
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("kube_role_version"),
			&stateKubeRoleVersion)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !kubeRoleVersion.Equal(stateKubeRoleVersion) {
			var upgradeToKubeRoleVersion types.String
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
				tflog.Debug(ctx, "upgrade_kube_role_version is not found in the plan")
				// This happens when API does not return the next available upgrade version.
				// API returns upgrade versions only when the cluster is in a state to be upgraded.
				// Because of this state does not contain next available upgrade version.
				// TODO: Find workaround, for example call getCluster here and check if it can
				// be upgraded.
				// resp.Diagnostics.AddError("Refresh local state", "Cluster is currently being upgraded or local state is out of date")
				// return
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
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID
	// TODO: Save intermediate state to prevent inconsistency between local and remote state
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
	if !data.WorkerNodes.IsNull() && !data.WorkerNodes.IsUnknown() {
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
	}
	nodesToAttachIDs := []string{}
	nodesToAttachIDs = append(nodesToAttachIDs, masterNodeIDs...)
	nodesToAttachIDs = append(nodesToAttachIDs, workerNodeIDs...)
	qbertNodesMap, err := r.getQbertNodesMap(projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get qbert nodes", err.Error())
		return
	}
	err = r.verifyNodesForAttach(ctx, nodesToAttachIDs, qbertNodesMap)
	if err != nil {
		resp.Diagnostics.AddError("Failed to verify nodes", err.Error())
		return
	}
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

	jsonRequest, err := json.Marshal(createClusterReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to marshal createClusterRequest", err.Error())
		return
	}
	strRequest := string(jsonRequest)
	tflog.Info(ctx, "Creating a cluster", map[string]interface{}{"request": strRequest})
	clusterID, err := r.client.Qbert().CreateCluster(createClusterReq, projectID, qbert.CreateClusterOptions{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cluster", err.Error())
		return
	}

	tflog.Info(ctx, "Cluster created", map[string]interface{}{"clusterID": clusterID})

	tflog.Info(ctx, "Attaching nodes", map[string]interface{}{"nodeList": nodeList})
	err = r.client.Qbert().AttachNodes(clusterID, nodeList)
	if err != nil {
		resp.Diagnostics.AddError("Failed to attach nodes", err.Error())
		return
	}
	workerNodesSetVal, diags := types.SetValueFrom(ctx, types.StringType, workerNodeIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("worker_nodes"), workerNodesSetVal)...)
	if resp.Diagnostics.HasError() {
		return
	}
	masterNodesSetVal, diags := types.SetValueFrom(ctx, types.StringType, masterNodeIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("master_nodes"), masterNodesSetVal)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.WorkerNodes = workerNodesSetVal
	state.MasterNodes = masterNodesSetVal

	if !data.Addons.IsNull() && !data.Addons.IsUnknown() {
		// This is a workaround because default addons are not being set in the plan.
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
		var addonsFromPlan types.Map
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("addons"), &addonsFromPlan)...)
		// resp.Diagnostics.Append(data.Addons.ElementsAs(ctx, &addonsFromPlan, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !addonsFromPlan.IsNull() && !addonsFromPlan.IsUnknown() {
			tfAddonsMap := map[string]resource_cluster.AddonsValue{}
			resp.Diagnostics.Append(addonsFromPlan.ElementsAs(ctx, &tfAddonsMap, false)...)
			for addonName, tfAddon := range tfAddonsMap {
				// sunpikeAddon represents remote state and tfAddon represents plan state
				if sunpikeAddon, found := sunpikeAddonsMap[addonName]; found && !tfAddon.IsNull() {
					// Case 1:
					// if addon with the same name is available at both places, difference bw
					// the two should be patched, prefering the plan instance.
					if !tfAddon.Enabled.IsNull() && !tfAddon.Enabled.IsUnknown() && !tfAddon.Enabled.ValueBool() {
						tflog.Debug(ctx, "Disabling addon because enabled=false", map[string]interface{}{"addon": addonName})
						err = r.addonsClient.Disable(ctx, AddonSpec{ClusterID: clusterID, Type: addonName})
						if err != nil {
							resp.Diagnostics.AddError("Failed to disable addon", err.Error())
							return
						}
						continue
					}
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
					tfParamsInPlan := map[string]types.String{}
					if !tfAddon.Params.IsUnknown() {
						resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &tfParamsInPlan, false)...)
						if resp.Diagnostics.HasError() {
							return
						}
					}
					paramsInPlan := map[string]string{}
					for key, value := range tfParamsInPlan {
						paramsInPlan[key] = value.ValueString()
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
					tfParamsInPlan := map[string]types.String{}
					if !tfAddon.Params.IsUnknown() {
						resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &tfParamsInPlan, false)...)
						if resp.Diagnostics.HasError() {
							return
						}
					}
					paramsInPlan := map[string]string{}
					for key, value := range tfParamsInPlan {
						// TODO: Decide how to handle null values. Currently they are sent as empty strings.
						paramsInPlan[key] = value.ValueString()
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
	tfAddonsMapState, diags := sunpikeAddonsToTerraformAddons(ctx, addonsOnRemote, data.Addons)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	tfAddonsRemote, diags := types.MapValueFrom(ctx, resource_cluster.AddonsValue{}.Type(ctx), tfAddonsMapState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Addons = tfAddonsRemote
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
	if clusterID == "" {
		resp.Diagnostics.AddError("Cluster ID is not provided", "Cluster ID is required to read the cluster")
		return
	}
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
	tfAddonsMapState, diags := sunpikeAddonsToTerraformAddons(ctx, addonsOnRemote, data.Addons)
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
	if (!plan.WorkerNodes.IsUnknown() && !plan.WorkerNodes.Equal(state.WorkerNodes)) || !plan.MasterNodes.Equal(state.MasterNodes) {
		tflog.Debug(ctx, "Change in nodes detected, attaching/detaching nodes")
		resp.Diagnostics.Append(r.attachDetachNodes(ctx, clusterID, projectID, plan, state)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	editClusterReq := qbert.EditClusterRequest{}
	var editRequired bool
	if !plan.EtcdBackup.Equal(state.EtcdBackup) {
		editRequired = true
		etcdBackupConfig, convertDiags := getEtcdBackupConfig(ctx, plan.EtcdBackup)
		resp.Diagnostics.Append(convertDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		editClusterReq.EtcdBackup = etcdBackupConfig
	}

	if !plan.CertExpiryHrs.Equal(state.CertExpiryHrs) {
		editRequired = true
		editClusterReq.CertExpiryHrs = int(plan.CertExpiryHrs.ValueInt64())
	}

	if !plan.CustomRegistry.Equal(state.CustomRegistry) {
		editRequired = true
		editClusterReq.CustomRegistryURL = plan.CustomRegistry.Url.ValueString()
		editClusterReq.CustomRegistryRepoPath = plan.CustomRegistry.RepoPath.ValueString()
		editClusterReq.CustomRegistryUsername = plan.CustomRegistry.Username.ValueString()
		editClusterReq.CustomRegistryPassword = plan.CustomRegistry.Password.ValueString()
		editClusterReq.CustomRegistryCertPath = plan.CustomRegistry.CertPath.ValueString()
		editClusterReq.CustomRegistrySkipTLS = getIntPtrFromBool(plan.CustomRegistry.SkipTls)
		editClusterReq.CustomRegistrySelfSignedCerts = getIntPtrFromBool(plan.CustomRegistry.SelfSignedCerts)
	}

	if !plan.EnableCatapultMonitoring.Equal(state.EnableCatapultMonitoring) {
		editRequired = true
		if !plan.EnableCatapultMonitoring.IsNull() && !plan.EnableCatapultMonitoring.IsUnknown() {
			editClusterReq.EnableCatapultMonitoring = plan.EnableCatapultMonitoring.ValueBoolPointer()
		}
	}

	if !plan.Tags.Equal(state.Tags) && !plan.Tags.IsUnknown() {
		if !plan.Tags.IsNull() {
			editRequired = true
			planTagsMap := map[string]string{}
			resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &planTagsMap, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			editClusterReq.Tags = planTagsMap
		} else {
			editRequired = true
			editClusterReq.Tags = map[string]string{}
		}
	}

	if editRequired {
		jsonRequest, err := json.Marshal(editClusterReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to marshal editClusterReq", err.Error())
			return
		}
		strRequest := string(jsonRequest)
		tflog.Info(ctx, "Editing cluster", map[string]interface{}{"request": strRequest})
		err = r.client.Qbert().EditCluster(editClusterReq, clusterID, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update cluster", err.Error())
			return
		}
	} else {
		tflog.Debug(ctx, "No change detected, skipping edit cluster")
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
	if !plan.Addons.IsNull() && !plan.Addons.IsUnknown() {
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
			if sunpikeAddon, found := sunpikeAddonsMap[addonName]; found && !tfAddon.IsNull() {
				if !tfAddon.Enabled.IsNull() && !tfAddon.Enabled.IsUnknown() && !tfAddon.Enabled.ValueBool() {
					tflog.Debug(ctx, "Disabling addon because enabled=false", map[string]interface{}{"addon": addonName})
					err = r.addonsClient.Disable(ctx, AddonSpec{ClusterID: clusterID, Type: addonName})
					if err != nil {
						resp.Diagnostics.AddError("Failed to disable addon", err.Error())
						return
					}
					continue
				}
				// Patch the addon
				tflog.Debug(ctx, "Checking if addon version and params needs to be patched")
				var addonVersion string
				if tfAddon.Version.IsNull() || tfAddon.Version.IsUnknown() {
					tflog.Debug(ctx, "Version is not provided in the plan, getting default version")
					addonVersion = getDefaultAddonVersion(defaultAddonVersions, addonName)
				} else {
					addonVersion = tfAddon.Version.ValueString()
				}
				tfParamsInPlan := map[string]types.String{}
				if !tfAddon.Params.IsUnknown() {
					resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &tfParamsInPlan, false)...)
					if resp.Diagnostics.HasError() {
						return
					}
				}
				paramsInPlan := map[string]string{}
				for key, value := range tfParamsInPlan {
					paramsInPlan[key] = value.ValueString()
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
			} else if tfAddon.IsNull() {
				// Disable the addon because it is null in the plan
				tflog.Debug(ctx, "Disabling addon", map[string]interface{}{"addon": addonName})
				err = r.addonsClient.Disable(ctx, AddonSpec{ClusterID: clusterID, Type: addonName})
				if err != nil {
					resp.Diagnostics.AddError("Failed to disable addon", err.Error())
					return
				}
			} else {
				// Enable the addon
				tflog.Debug(ctx, "Enabling addon", map[string]interface{}{"addon": addonName})
				tfParamsInPlan := map[string]types.String{}
				if !tfAddon.Params.IsUnknown() {
					resp.Diagnostics.Append(tfAddon.Params.ElementsAs(ctx, &tfParamsInPlan, false)...)
					if resp.Diagnostics.HasError() {
						return
					}
				}
				paramsInPlan := map[string]string{}
				for key, value := range tfParamsInPlan {
					paramsInPlan[key] = value.ValueString()
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
	}

	if !plan.KubeRoleVersion.Equal(state.KubeRoleVersion) {
		tflog.Debug(ctx, "Requested upgrade of the cluster", map[string]interface{}{"from": state.KubeRoleVersion, "to": plan.KubeRoleVersion})
		tflog.Debug(ctx, "Reading cluster from qbert", map[string]interface{}{"clusterID": clusterID})
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
		jsonRequest, err := json.Marshal(upgradeClusterReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to marshal upgradeClusterReq", err.Error())
			return
		}
		strRequest := string(jsonRequest)
		tflog.Info(ctx, "Upgrading a cluster", map[string]interface{}{"request": strRequest, "clusterID": clusterID,
			"type": upgradeClusterReq.KubeRoleVersionUpgradeType})
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
	tfAddonsMapState, diags := sunpikeAddonsToTerraformAddons(ctx, sunpikeAddons, plan.Addons)
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

	tflog.Info(ctx, "Reading cluster", map[string]interface{}{"clusterID": clusterID})
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
	state.MasterNodes, diags = types.SetValueFrom(ctx, types.StringType, masterNodes)
	if diags.HasError() {
		return diags
	}
	if len(workerNodes) > 0 {
		state.WorkerNodes, diags = types.SetValueFrom(ctx, types.StringType, workerNodes)
		if diags.HasError() {
			return diags
		}
	} else {
		state.WorkerNodes = types.SetNull(types.StringType)
	}
	return diags
}

func (r *clusterResource) attachDetachNodes(ctx context.Context, clusterID string, projectID string, plan resource_cluster.ClusterModel, state resource_cluster.ClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics
	nodeList := []qbert.Node{}
	masterNodesFromPlan := []types.String{}
	qbertNodesMap, err := r.getQbertNodesMap(projectID)
	if err != nil {
		diags.AddError("Failed to get qbert nodes", err.Error())
		return diags
	}
	if !plan.MasterNodes.IsNull() && !plan.MasterNodes.IsUnknown() {
		diags.Append(plan.MasterNodes.ElementsAs(ctx, &masterNodesFromPlan, false)...)
		if diags.HasError() {
			return diags
		}
	}
	masterNodesFromState := []types.String{}
	if !state.MasterNodes.IsNull() && !state.MasterNodes.IsUnknown() {
		diags.Append(state.MasterNodes.ElementsAs(ctx, &masterNodesFromState, false)...)
		if diags.HasError() {
			return diags
		}
	}
	strMasterNodesFromState := []string{}
	for _, nodeID := range masterNodesFromState {
		if !nodeID.IsNull() && !nodeID.IsUnknown() {
			strMasterNodesFromState = append(strMasterNodesFromState, nodeID.ValueString())
		}
	}
	strMasterNodesFromPlan := []string{}
	for _, nodeID := range masterNodesFromPlan {
		if !nodeID.IsNull() && !nodeID.IsUnknown() {
			strMasterNodesFromPlan = append(strMasterNodesFromPlan, nodeID.ValueString())
		}
	}
	diffMasters := findDiff(strMasterNodesFromState, strMasterNodesFromPlan)
	for _, nodeID := range diffMasters.Removed {
		if qbertNodesMap[nodeID].ClusterUUID == "" {
			tflog.Debug(ctx, "Node is already detached", map[string]interface{}{"nodeID": nodeID})
			continue
		}
		nodeList = append(nodeList, qbert.Node{
			UUID: nodeID,
		})
	}

	workerNodesFromPlan := []types.String{}
	if !plan.WorkerNodes.IsNull() && !plan.WorkerNodes.IsUnknown() {
		diags.Append(plan.WorkerNodes.ElementsAs(ctx, &workerNodesFromPlan, false)...)
		if diags.HasError() {
			return diags
		}
	}
	workerNodesFromState := []types.String{}
	if !state.WorkerNodes.IsNull() && !state.WorkerNodes.IsUnknown() {
		diags.Append(state.WorkerNodes.ElementsAs(ctx, &workerNodesFromState, false)...)
		if diags.HasError() {
			return diags
		}
	}
	strWorkerNodesFromState := []string{}
	for _, nodeID := range workerNodesFromState {
		if !nodeID.IsNull() && !nodeID.IsUnknown() {
			strWorkerNodesFromState = append(strWorkerNodesFromState, nodeID.ValueString())
		}
	}
	strWorkerNodesFromPlan := []string{}
	for _, nodeID := range workerNodesFromPlan {
		if !nodeID.IsNull() && !nodeID.IsUnknown() {
			strWorkerNodesFromPlan = append(strWorkerNodesFromPlan, nodeID.ValueString())
		}
	}
	diffWorkers := findDiff(strWorkerNodesFromState, strWorkerNodesFromPlan)
	for _, nodeID := range diffWorkers.Removed {
		if node, found := qbertNodesMap[nodeID]; !found || node.ClusterUUID == "" {
			tflog.Debug(ctx, "Node is already detached", map[string]interface{}{"nodeID": nodeID, "nodeFound": found})
			continue
		}
		nodeList = append(nodeList, qbert.Node{
			UUID: nodeID,
		})
	}
	if len(nodeList) > 0 {
		tflog.Debug(ctx, "Detaching nodes", map[string]interface{}{"nodeList": nodeList})
		err := r.client.Qbert().DetachNodes(clusterID, nodeList)
		if err != nil {
			diags.AddError("Failed to detach nodes", err.Error())
			return diags
		}
	}
	nodesToAttachIDs := []string{}
	nodeList = []qbert.Node{}
	for _, nodeID := range diffMasters.Added {
		if isNodeAlreadyAttached(clusterID, qbertNodesMap[nodeID], true) {
			tflog.Debug(ctx, "Node is already attached as master", map[string]interface{}{"nodeID": nodeID})
			continue
		}
		nodeList = append(nodeList, qbert.Node{
			UUID:     nodeID,
			IsMaster: 1,
		})
		nodesToAttachIDs = append(nodesToAttachIDs, nodeID)
	}
	for _, nodeID := range diffWorkers.Added {
		if isNodeAlreadyAttached(clusterID, qbertNodesMap[nodeID], false) {
			tflog.Debug(ctx, "Node is already attached as worker", map[string]interface{}{"nodeID": nodeID})
			continue
		}
		nodeList = append(nodeList, qbert.Node{
			UUID:     nodeID,
			IsMaster: 0,
		})
		nodesToAttachIDs = append(nodesToAttachIDs, nodeID)
	}
	err = r.verifyNodesForAttach(ctx, nodesToAttachIDs, qbertNodesMap)
	if err != nil {
		diags.AddError("Failed to verify nodes are eligible to attach", err.Error())
		return diags
	}
	if len(nodeList) > 0 {
		tflog.Debug(ctx, "Attaching nodes", map[string]interface{}{"nodeList": nodeList})
		err := r.client.Qbert().AttachNodes(clusterID, nodeList)
		if err != nil {
			diags.AddError("Failed to attach nodes", err.Error())
			return diags
		}
	}

	return diags
}

func isNodeAlreadyAttached(clusterID string, node qbert.Node, isMaster bool) bool {
	return node.ClusterUUID == clusterID && ((isMaster && node.IsMaster == 1) || (!isMaster && node.IsMaster == 0))
}

func (r *clusterResource) getQbertNodesMap(projectID string) (map[string]qbert.Node, error) {
	nodesMap := map[string]qbert.Node{}
	qbertNodes, err := r.client.Qbert().ListNodes(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get qbert nodes: %w", err)
	}
	for _, node := range qbertNodes {
		nodesMap[node.UUID] = node
	}
	return nodesMap, nil
}

func (r *clusterResource) verifyNodesForAttach(ctx context.Context, nodesToAttachIDs []string, qbertNodesMap map[string]qbert.Node) error {
	tflog.Debug(ctx, "Checking if nodes can be attached", map[string]interface{}{"nodesToAttachIDs": nodesToAttachIDs})
	for _, nodeID := range nodesToAttachIDs {
		node, found := qbertNodesMap[nodeID]
		if !found {
			return fmt.Errorf("node %v not found", nodeID)
		}
		if node.Status != "ok" {
			return fmt.Errorf("node %v is not in a 'ok' state. Current state:%v", nodeID, node.Status)
		}
		if node.ClusterName != "" {
			return fmt.Errorf("node %v is already attached to a cluster %v", nodeID, node.ClusterName)
		}
	}
	return nil
}

func getParamsMapForAllAddons(ctx context.Context, planAddons types.Map) (map[string]map[string]types.String, diag.Diagnostics) {
	if !planAddons.IsNull() && !planAddons.IsUnknown() {
		tfAddonsMap := map[string]resource_cluster.AddonsValue{}
		var diags diag.Diagnostics
		diags = planAddons.ElementsAs(ctx, &tfAddonsMap, false)
		if diags.HasError() {
			return nil, diags
		}
		paramsMap := map[string]map[string]types.String{}
		for addonName, tfAddon := range tfAddonsMap {
			if !tfAddon.Params.IsUnknown() {
				params := map[string]types.String{}
				diags = tfAddon.Params.ElementsAs(ctx, &params, false)
				if diags.HasError() {
					return nil, diags
				}
				paramsMap[addonName] = params
			}
		}
		return paramsMap, diags
	}
	return nil, nil
}

func sunpikeAddonsToTerraformAddons(ctx context.Context, sunpikeAddons []sunpikev1alpha2.ClusterAddon, planAddons types.Map,
) (map[string]resource_cluster.AddonsValue, diag.Diagnostics) {
	tfAddonsMap := map[string]resource_cluster.AddonsValue{}
	var diags diag.Diagnostics
	// know which param was null and which was empty in plan
	paramsForAllAddons, diags := getParamsMapForAllAddons(ctx, planAddons)
	if diags.HasError() {
		return tfAddonsMap, diags
	}
	for _, sunpikeAddon := range sunpikeAddons {
		addonName := sunpikeAddon.Spec.Type
		version := types.StringValue(sunpikeAddon.Spec.Version)
		phase := types.StringValue(string(sunpikeAddon.Status.Phase))
		paramMap := map[string]types.String{}
		for _, param := range sunpikeAddon.Spec.Override.Params {
			if param.Value == "" {
				// API always returns empty for non provided params, but terraform users
				// can either provide null or empty string, and we need to keep the state
				// same as plan, if param is null in plan, we need to keep it null in state
				paramMap[param.Name] = paramsForAllAddons[addonName][param.Name]
				continue
			}
			paramMap[param.Name] = types.StringValue(param.Value)
		}
		var params types.Map
		params, diags = types.MapValueFrom(ctx, types.StringType, paramMap)
		if diags.HasError() {
			return tfAddonsMap, diags
		}
		addonObjVal, diags := resource_cluster.AddonsValue{
			Enabled: types.BoolValue(true),
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
	if !planAddons.IsNull() && !planAddons.IsUnknown() {
		tfPlanAddonsMap := map[string]resource_cluster.AddonsValue{}
		diags := planAddons.ElementsAs(ctx, &tfPlanAddonsMap, false)
		if diags.HasError() {
			return tfAddonsMap, diags
		}
		for addonName, tfPlanAddon := range tfPlanAddonsMap {
			if _, found := tfAddonsMap[addonName]; !found {
				if tfPlanAddon.IsNull() {
					// plan has addon=null, to make remote state same as plan we need to mark it as null in the state too
					tfAddonsMap[addonName] = resource_cluster.NewAddonsValueNull()
				}
				if !tfPlanAddon.Enabled.IsNull() && !tfPlanAddon.Enabled.IsUnknown() && !tfPlanAddon.Enabled.ValueBool() {
					// plan has addon enabled=false, to make remote state same as plan, copy plan addon to state
					tmpParams := types.MapNull(types.StringType)
					if !tfPlanAddon.Params.IsUnknown() {
						tmpParams = tfPlanAddon.Params
					}
					addonObjVal, convertDiags := resource_cluster.AddonsValue{
						Enabled: types.BoolValue(false),
						Params:  tmpParams,
						Phase:   types.StringNull(),
						Version: tfPlanAddon.Version,
					}.ToObjectValue(ctx)
					diags.Append(convertDiags...)
					if diags.HasError() {
						return tfAddonsMap, diags
					}
					addonVal, convertDiags := resource_cluster.NewAddonsValue(addonObjVal.AttributeTypes(ctx), addonObjVal.Attributes())
					diags.Append(convertDiags...)
					if diags.HasError() {
						return tfAddonsMap, diags
					}
					tfAddonsMap[addonName] = addonVal
				}
			}
		}
	}
	return tfAddonsMap, diags
}

func qbertClusterToTerraformCluster(ctx context.Context, qbertCluster *qbert.Cluster, clusterModel *resource_cluster.ClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics
	clusterModel.Id = types.StringValue(qbertCluster.UUID)
	clusterModel.Name = types.StringValue(qbertCluster.Name)
	clusterModel.AllowWorkloadsOnMaster = types.BoolValue(qbertCluster.AllowWorkloadsOnMaster != 0)
	clusterModel.MasterIp = types.StringValue(qbertCluster.MasterIP)
	clusterModel.MasterVipIface = types.StringValue(qbertCluster.MasterVipIface)
	clusterModel.MasterVipIpv4 = types.StringValue(qbertCluster.MasterVipIpv4)
	clusterModel.MasterVipVrouterId = types.StringValue(qbertCluster.MasterVipVrouterID)
	clusterModel.ContainersCidr = types.StringValue(qbertCluster.ContainersCidr)
	clusterModel.ServicesCidr = types.StringValue(qbertCluster.ServicesCidr)
	clusterModel.Privileged = types.BoolValue(qbertCluster.Privileged != 0)
	clusterModel.UseHostname = types.BoolValue(qbertCluster.UseHostname)
	clusterModel.InterfaceName = types.StringValue(qbertCluster.InterfaceName)
	clusterModel.InterfaceReachableIp = types.StringValue(qbertCluster.InterfaceReachableIP)
	clusterModel.InterfaceDetectionMethod = types.StringValue(qbertCluster.InterfaceDetectionMethod)
	clusterModel.NodePoolUuid = types.StringValue(qbertCluster.NodePoolUUID)
	clusterModel.NodePoolName = types.StringValue(qbertCluster.NodePoolName)

	mtuSizeInt, err := strconv.Atoi(qbertCluster.MtuSize)
	if err != nil {
		diags.AddError("Failed to parse mtu size", err.Error())
		return diags
	}
	clusterModel.MtuSize = types.Int64Value(int64(mtuSizeInt))

	clusterModel.ContainerRuntime = types.StringValue(qbertCluster.ContainerRuntime)
	clusterModel.DockerRoot = types.StringValue(qbertCluster.DockerRoot)
	clusterModel.FelixIpv6Support = types.BoolValue(qbertCluster.FelixIPv6Support != 0)
	clusterModel.Masterless = types.BoolValue(qbertCluster.Masterless != 0)
	clusterModel.ExternalDnsName = getStrOrNullIfEmpty(qbertCluster.ExternalDNSName)
	clusterModel.CertExpiryHrs = types.Int64Value(int64(qbertCluster.CertExpiryHrs))
	clusterModel.Ipv6 = types.BoolValue(qbertCluster.IPv6 != 0)

	clusterModel.CpuManagerPolicy = types.StringValue(qbertCluster.CPUManagerPolicy)
	clusterModel.TopologyManagerPolicy = types.StringValue(qbertCluster.TopologyManagerPolicy)
	clusterModel.ReservedCpus = getStrOrNullIfEmpty(qbertCluster.ReservedCPUs)

	clusterModel.NetworkPlugin = types.StringValue(qbertCluster.NetworkPlugin)

	clusterModel.FlannelIfaceLabel = getStrOrNullIfEmpty(qbertCluster.FlannelIfaceLabel)
	clusterModel.FlannelPublicIfaceLabel = getStrOrNullIfEmpty(qbertCluster.FlannelPublicIfaceLabel)

	clusterModel.CalicoIpv4 = types.StringValue(qbertCluster.CalicoIPv4)
	clusterModel.CalicoIpv6 = types.StringValue(qbertCluster.CalicoIPv6)
	clusterModel.CalicoIpv6DetectionMethod = types.StringValue(qbertCluster.CalicoIPv6DetectionMethod)
	clusterModel.CalicoRouterId = types.StringValue(qbertCluster.CalicoRouterID)
	clusterModel.CalicoIpv6PoolCidr = getStrOrNullIfEmpty(qbertCluster.CalicoIPv6PoolCidr)
	clusterModel.CalicoIpv6PoolBlockSize = types.StringValue(qbertCluster.CalicoIPv6PoolBlockSize)
	clusterModel.CalicoIpv6PoolNatOutgoing = types.BoolValue(qbertCluster.CalicoIPv6PoolNatOutgoing != 0)
	clusterModel.CalicoIpIpMode = types.StringValue(qbertCluster.CalicoIPIPMode)
	clusterModel.CalicoNatOutgoing = types.BoolValue(qbertCluster.CalicoNatOutgoing != 0)
	clusterModel.CalicoV4BlockSize = types.StringValue(qbertCluster.CalicoV4BlockSize)
	clusterModel.CalicoIpv4DetectionMethod = types.StringValue(qbertCluster.CalicoIPv4DetectionMethod)

	clusterModel.DockerPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.DockerPrivateRegistry)
	clusterModel.QuayPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.QuayPrivateRegistry)
	clusterModel.GcrPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.GcrPrivateRegistry)
	clusterModel.K8sPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.K8sPrivateRegistry)
	clusterModel.DockerCentosPackageRepoUrl = getStrOrNullIfEmpty(qbertCluster.DockerCentosPackageRepoURL)
	clusterModel.DockerUbuntuPackageRepoUrl = getStrOrNullIfEmpty(qbertCluster.DockerUbuntuPackageRepoURL)

	clusterModel.CreatedAt = types.StringValue(qbertCluster.CreatedAt)
	clusterModel.ProjectId = types.StringValue(qbertCluster.ProjectID)

	// KubeRoleVersion does not change immediately after cluster upgrade
	// hence this is a workaround to get the correct value
	if qbertCluster.UpgradingTo != "" {
		clusterModel.KubeRoleVersion = types.StringValue(qbertCluster.UpgradingTo)
	} else {
		clusterModel.KubeRoleVersion = types.StringValue(qbertCluster.KubeRoleVersion)
	}
	if qbertCluster.K8sAPIPort == "" {
		clusterModel.K8sApiPort = types.Int64Null()
	} else {
		intPort, err := strconv.Atoi(qbertCluster.K8sAPIPort)
		if err != nil {
			diags.AddError("Failed to parse k8s api port", err.Error())
			return diags
		}
		clusterModel.K8sApiPort = types.Int64Value(int64(intPort))
	}
	if qbertCluster.EnableCatapultMonitoring != nil {
		clusterModel.EnableCatapultMonitoring = types.BoolValue(*qbertCluster.EnableCatapultMonitoring)
	}
	k8sconfig, convertDiags := getK8sConfigValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.K8sConfig = k8sconfig

	calicoLimits, convertDiags := getCalicoLimitsValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.CalicoLimits = calicoLimits

	// Computed attributes
	statusValue, convertDiags := getStatusValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.Status = statusValue

	cloudProviderValue, convertDiags := getCloudProviderValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.CloudProvider = cloudProviderValue

	customRegistry, convertDiags := getCustomRegistryValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.CustomRegistry = customRegistry

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

	etcdValue, convertDiags := getEtcdValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.Etcd = etcdValue
	etcdBackupValue, convertDiags := getEtcdBackupValue(ctx, qbertCluster.EtcdBackup)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.EtcdBackup = etcdBackupValue

	if len(qbertCluster.Tags) == 0 {
		clusterModel.Tags = types.MapNull(types.StringType)
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
	}
	createClusterReq.Name = clusterModel.Name.ValueString()
	createClusterReq.MasterIP = clusterModel.MasterIp.ValueString()
	if !clusterModel.AllowWorkloadsOnMaster.IsNull() && !clusterModel.AllowWorkloadsOnMaster.IsUnknown() {
		createClusterReq.AllowWorkloadOnMaster = clusterModel.AllowWorkloadsOnMaster.ValueBoolPointer()
	}
	createClusterReq.MasterVirtualIPIface = clusterModel.MasterVipIface.ValueString()
	createClusterReq.MasterVirtualIP = clusterModel.MasterVipIpv4.ValueString()
	createClusterReq.ContainerCIDR = clusterModel.ContainersCidr.ValueString()
	createClusterReq.ServiceCIDR = clusterModel.ServicesCidr.ValueString()
	createClusterReq.MTUSize = ptr.To(int(clusterModel.MtuSize.ValueInt64()))
	if !clusterModel.Privileged.IsNull() && !clusterModel.Privileged.IsUnknown() {
		createClusterReq.Privileged = clusterModel.Privileged.ValueBoolPointer()
	}
	if !clusterModel.UseHostname.IsNull() && !clusterModel.UseHostname.IsUnknown() {
		createClusterReq.UseHostname = clusterModel.UseHostname.ValueBoolPointer()
	}
	createClusterReq.InterfaceDetectionMethod = clusterModel.InterfaceDetectionMethod.ValueString()
	createClusterReq.InterfaceName = clusterModel.InterfaceName.ValueString()
	createClusterReq.InterfaceReachableIP = clusterModel.InterfaceReachableIp.ValueString()
	createClusterReq.KubeRoleVersion = clusterModel.KubeRoleVersion.ValueString()
	createClusterReq.CPUManagerPolicy = clusterModel.CpuManagerPolicy.ValueString()
	createClusterReq.ExternalDNSName = clusterModel.ExternalDnsName.ValueString()
	createClusterReq.TopologyManagerPolicy = clusterModel.TopologyManagerPolicy.ValueString()
	createClusterReq.ReservedCPUs = clusterModel.ReservedCpus.ValueString()
	createClusterReq.ContainerRuntime = qbert.ContainerRuntime(clusterModel.ContainerRuntime.ValueString())
	createClusterReq.DockerRoot = clusterModel.DockerRoot.ValueString()
	createClusterReq.IPv6 = getIntPtrFromBool(clusterModel.Ipv6)
	createClusterReq.FelixIPv6Support = getIntPtrFromBool(clusterModel.FelixIpv6Support)

	createClusterReq.NetworkPlugin = qbert.CNIBackend(clusterModel.NetworkPlugin.ValueString())
	createClusterReq.FlannelIfaceLabel = clusterModel.FlannelIfaceLabel.ValueString()
	createClusterReq.FlannelPublicIfaceLabel = clusterModel.FlannelPublicIfaceLabel.ValueString()
	createClusterReq.CalicoIPIPMode = clusterModel.CalicoIpIpMode.ValueString()
	// The qbert api accepts this field as bool but returns as int
	if !clusterModel.CalicoNatOutgoing.IsNull() && !clusterModel.CalicoNatOutgoing.IsUnknown() {
		createClusterReq.CalicoNatOutgoing = clusterModel.CalicoNatOutgoing.ValueBoolPointer()
	}
	createClusterReq.CalicoV4BlockSize = clusterModel.CalicoV4BlockSize.ValueString()
	createClusterReq.CalicoIpv4 = clusterModel.CalicoIpv4.ValueString()
	createClusterReq.CalicoIpv4DetectionMethod = clusterModel.CalicoIpv4DetectionMethod.ValueString()

	createClusterReq.CalicoIPv6 = clusterModel.CalicoIpv6.ValueString()
	createClusterReq.CalicoIPv6DetectionMethod = clusterModel.CalicoIpv6DetectionMethod.ValueString()
	createClusterReq.CalicoIPv6PoolCidr = clusterModel.CalicoIpv6PoolCidr.ValueString()
	createClusterReq.CalicoIPv6PoolBlockSize = clusterModel.CalicoIpv6PoolBlockSize.ValueString()
	createClusterReq.CalicoIPv6PoolNatOutgoing = getIntPtrFromBool(clusterModel.CalicoIpv6PoolNatOutgoing)

	createClusterReq.CalicoNodeCPULimit = clusterModel.CalicoLimits.NodeCpuLimit.ValueString()
	createClusterReq.CalicoNodeMemoryLimit = clusterModel.CalicoLimits.NodeMemoryLimit.ValueString()
	createClusterReq.CalicoTyphaCPULimit = clusterModel.CalicoLimits.TyphaCpuLimit.ValueString()
	createClusterReq.CalicoTyphaMemoryLimit = clusterModel.CalicoLimits.TyphaMemoryLimit.ValueString()
	createClusterReq.CalicoControllerCPULimit = clusterModel.CalicoLimits.ControllerCpuLimit.ValueString()
	createClusterReq.CalicoControllerMemoryLimit = clusterModel.CalicoLimits.ControllerMemoryLimit.ValueString()

	createClusterReq.DockerPrivateRegistry = clusterModel.DockerPrivateRegistry.ValueString()
	createClusterReq.QuayPrivateRegistry = clusterModel.QuayPrivateRegistry.ValueString()
	createClusterReq.GcrPrivateRegistry = clusterModel.GcrPrivateRegistry.ValueString()
	createClusterReq.K8sPrivateRegistry = clusterModel.K8sPrivateRegistry.ValueString()
	createClusterReq.DockerCentosPackageRepoURL = clusterModel.DockerCentosPackageRepoUrl.ValueString()
	createClusterReq.DockerUbuntuPackageRepoURL = clusterModel.DockerUbuntuPackageRepoUrl.ValueString()

	if !clusterModel.CustomRegistry.IsNull() && !clusterModel.CustomRegistry.IsUnknown() {
		if !clusterModel.CustomRegistry.Url.IsNull() && !clusterModel.CustomRegistry.Url.IsUnknown() {
			createClusterReq.CustomRegistryURL = clusterModel.CustomRegistry.Url.ValueString()
		}
		if !clusterModel.CustomRegistry.RepoPath.IsNull() && !clusterModel.CustomRegistry.RepoPath.IsUnknown() {
			createClusterReq.CustomRegistryRepoPath = clusterModel.CustomRegistry.RepoPath.ValueString()
		}
		if !clusterModel.CustomRegistry.Username.IsNull() && !clusterModel.CustomRegistry.Username.IsUnknown() {
			createClusterReq.CustomRegistryUsername = clusterModel.CustomRegistry.Username.ValueString()
		}
		if !clusterModel.CustomRegistry.Password.IsNull() && !clusterModel.CustomRegistry.Password.IsUnknown() {
			createClusterReq.CustomRegistryPassword = clusterModel.CustomRegistry.Password.ValueString()
		}
		if !clusterModel.CustomRegistry.CertPath.IsNull() && !clusterModel.CustomRegistry.CertPath.IsUnknown() {
			createClusterReq.CustomRegistryCertPath = clusterModel.CustomRegistry.CertPath.ValueString()
		}
		createClusterReq.CustomRegistrySkipTLS = getIntPtrFromBool(clusterModel.CustomRegistry.SkipTls)
		createClusterReq.CustomRegistrySelfSignedCerts = getIntPtrFromBool(clusterModel.CustomRegistry.SelfSignedCerts)
	}

	if !clusterModel.EnableCatapultMonitoring.IsNull() && !clusterModel.EnableCatapultMonitoring.IsUnknown() {
		createClusterReq.EnableCatapultMonitoring = clusterModel.EnableCatapultMonitoring.ValueBoolPointer()
	}
	if !clusterModel.Etcd.EnableEncryption.IsNull() && !clusterModel.Etcd.EnableEncryption.IsUnknown() {
		// TODO: This does not work, etcd encryption always gets enabled
		createClusterReq.EnableEtcdEncryption = fmt.Sprintf("%v", clusterModel.Etcd.EnableEncryption.ValueBool())
	}
	if !clusterModel.Etcd.DataDir.IsNull() && !clusterModel.Etcd.DataDir.IsUnknown() {
		createClusterReq.EtcdDataDir = clusterModel.Etcd.DataDir.ValueString()
	}
	if !clusterModel.Etcd.Version.IsNull() && !clusterModel.Etcd.Version.IsUnknown() {
		createClusterReq.EtcdVersion = clusterModel.Etcd.Version.ValueString()
	}
	if !clusterModel.Etcd.ElectionTimeoutMs.IsNull() && !clusterModel.Etcd.ElectionTimeoutMs.IsUnknown() {
		createClusterReq.EtcdElectionTimeoutMs = fmt.Sprintf("%d", clusterModel.Etcd.ElectionTimeoutMs.ValueInt64())
	}
	if !clusterModel.Etcd.HeartbeatIntervalMs.IsNull() && !clusterModel.Etcd.HeartbeatIntervalMs.IsUnknown() {
		createClusterReq.EtcdHeartbeatIntervalMs = fmt.Sprintf("%d", clusterModel.Etcd.HeartbeatIntervalMs.ValueInt64())
	}

	if !clusterModel.K8sConfig.IsNull() && !clusterModel.K8sConfig.IsUnknown() {
		if !clusterModel.K8sConfig.ApiServerRuntimeConfig.IsNull() && !clusterModel.K8sConfig.ApiServerRuntimeConfig.IsUnknown() {
			createClusterReq.RuntimeConfig = clusterModel.K8sConfig.ApiServerRuntimeConfig.ValueString()
		}
		cloudProperties, convertDiags := getCloudPropertiesValue(ctx, clusterModel.K8sConfig)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return createClusterReq, diags
		}
		createClusterReq.CloudProperties = cloudProperties
	}

	createClusterReq.EtcdBackup, diags = getEtcdBackupConfig(ctx, clusterModel.EtcdBackup)
	if diags.HasError() {
		return createClusterReq, diags
	}
	if !clusterModel.CertExpiryHrs.IsNull() && !clusterModel.CertExpiryHrs.IsUnknown() {
		createClusterReq.CertExpiryHrs = ptr.To(int(clusterModel.CertExpiryHrs.ValueInt64()))
	}

	if !clusterModel.K8sApiPort.IsNull() && !clusterModel.K8sApiPort.IsUnknown() {
		createClusterReq.KubeAPIPort = fmt.Sprintf("%d", clusterModel.K8sApiPort.ValueInt64())
	}

	if !clusterModel.Tags.IsNull() && !clusterModel.Tags.IsUnknown() {
		planTagsMap := map[string]string{}
		diags = clusterModel.Tags.ElementsAs(ctx, &planTagsMap, false)
		if diags.HasError() {
			return createClusterReq, diags
		}
		createClusterReq.Tags = planTagsMap
	}
	return createClusterReq, diags
}

func (r *clusterResource) listClusterAddons(ctx context.Context, clusterID string) ([]sunpikev1alpha2.ClusterAddon, error) {
	tflog.Info(ctx, "Listing addons", map[string]interface{}{"clusterID": clusterID})
	sunpikeAddonsList, err := r.client.Qbert().ListClusterAddons(fmt.Sprintf("sunpike.pf9.io/cluster=%s", clusterID))
	if err != nil {
		return nil, err
	}
	notDeletedAddons := []sunpikev1alpha2.ClusterAddon{}
	for _, addon := range sunpikeAddonsList.Items {
		if addon.DeletionTimestamp == nil {
			notDeletedAddons = append(notDeletedAddons, addon)
		}
	}
	return notDeletedAddons, nil
}

func getCloudPropertiesValue(ctx context.Context, k8sConfigValue resource_cluster.K8sConfigValue) (*qbert.CloudProperties, diag.Diagnostics) {
	var diags, convertDiags diag.Diagnostics
	cloudProperties := qbert.CloudProperties{}
	if !k8sConfigValue.IsNull() && !k8sConfigValue.IsUnknown() {
		cloudProperties.APIServerFlags, convertDiags = toJsonArrFromStrList(ctx, k8sConfigValue.ApiServerFlags)
		diags.Append(convertDiags...)
		cloudProperties.SchedulerFlags, convertDiags = toJsonArrFromStrList(ctx, k8sConfigValue.SchedulerFlags)
		diags.Append(convertDiags...)
		cloudProperties.ControllerManagerFlags, convertDiags = toJsonArrFromStrList(ctx, k8sConfigValue.ControllerManagerFlags)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return nil, diags
		}
	}
	if cloudProperties.APIServerFlags == "" && cloudProperties.SchedulerFlags == "" &&
		cloudProperties.ControllerManagerFlags == "" {
		return nil, diags
	}
	return &cloudProperties, diags
}

func getEtcdBackupConfig(ctx context.Context, etcdBackupValue resource_cluster.EtcdBackupValue) (*qbert.EtcdBackupConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	etcdBackupConfig := qbert.EtcdBackupConfig{}
	if etcdBackupValue.IsNull() {
		etcdBackupConfig.IsEtcdBackupEnabled = 0
	} else {
		etcdBackupConfig.IsEtcdBackupEnabled = 1
	}
	if !etcdBackupValue.Daily.IsNull() && !etcdBackupValue.Daily.IsUnknown() {
		dailyValue, convertDiags := resource_cluster.NewDailyValue(etcdBackupValue.Daily.AttributeTypes(ctx), etcdBackupValue.Daily.Attributes())
		diags.Append(convertDiags...)
		if diags.HasError() {
			return nil, diags
		}
		etcdBackupConfig.DailyBackupTime = dailyValue.BackupTime.ValueString()
		if !dailyValue.MaxBackupsToRetain.IsNull() && !dailyValue.MaxBackupsToRetain.IsUnknown() {
			etcdBackupConfig.MaxTimestampBackupCount = int(dailyValue.MaxBackupsToRetain.ValueInt64())
		}
	}
	if !etcdBackupValue.Interval.IsNull() && !etcdBackupValue.Interval.IsUnknown() {
		intervalValue, convertDiags := resource_cluster.NewIntervalValue(etcdBackupValue.Interval.AttributeTypes(ctx),
			etcdBackupValue.Interval.Attributes())
		diags.Append(convertDiags...)
		if diags.HasError() {
			return nil, diags
		}
		if !intervalValue.BackupInterval.IsNull() && !intervalValue.BackupInterval.IsUnknown() {
			if strBkpInterval, found := strings.CutSuffix(intervalValue.BackupInterval.ValueString(), "h"); found {
				intBackupInterval, err := strconv.Atoi(strBkpInterval)
				if err != nil {
					diags.AddError("Failed to parse backup intervalValue", err.Error())
					return nil, diags
				}
				etcdBackupConfig.IntervalInHours = int(intBackupInterval)
			} else if strBkpInterval, found := strings.CutSuffix(intervalValue.BackupInterval.ValueString(), "m"); found {
				intBackupInterval, err := strconv.Atoi(strBkpInterval)
				if err != nil {
					diags.AddError("Failed to parse backup interval", err.Error())
					return nil, diags
				}
				etcdBackupConfig.IntervalInMins = int(intBackupInterval)
			}
		}
		if !intervalValue.MaxBackupsToRetain.IsNull() && !intervalValue.MaxBackupsToRetain.IsUnknown() {
			etcdBackupConfig.MaxIntervalBackupCount = int(intervalValue.MaxBackupsToRetain.ValueInt64())
		}
	}
	if !etcdBackupValue.StorageLocalPath.IsNull() && !etcdBackupValue.StorageLocalPath.IsUnknown() {
		etcdBackupConfig.StorageProperties.LocalPath = etcdBackupValue.StorageLocalPath.ValueStringPointer()
	}
	etcdBackupConfig.StorageType = etcdBackupValue.StorageType.ValueString()
	return &etcdBackupConfig, diags
}

func getEtcdBackupValue(ctx context.Context, etcdBackupConfig *qbert.EtcdBackupConfig) (resource_cluster.EtcdBackupValue, diag.Diagnostics) {
	etcdBackupValue := resource_cluster.EtcdBackupValue{}
	var diags diag.Diagnostics
	if etcdBackupConfig != nil && etcdBackupConfig.IsEtcdBackupEnabled == 1 {
		var dailyObjVal, intervalObjVal types.Object
		var convertDiags diag.Diagnostics
		if etcdBackupConfig.DailyBackupTime != "" {
			dailyObjVal, convertDiags = resource_cluster.DailyValue{
				BackupTime:         getStrOrNullIfEmpty(etcdBackupConfig.DailyBackupTime),
				MaxBackupsToRetain: getIntOrNullIfZero(etcdBackupConfig.MaxTimestampBackupCount),
			}.ToObjectValue(ctx)
			diags.Append(convertDiags...)
			if diags.HasError() {
				return etcdBackupValue, diags
			}
		} else {
			dailyObjVal = types.ObjectNull(resource_cluster.DailyValue{}.AttributeTypes(ctx))
		}

		if etcdBackupConfig.IntervalInHours != 0 || etcdBackupConfig.IntervalInMins != 0 {
			var backupIntervalVal string
			if etcdBackupConfig.IntervalInHours != 0 {
				backupIntervalVal = fmt.Sprintf("%dh", etcdBackupConfig.IntervalInHours)
			} else if etcdBackupConfig.IntervalInMins != 0 {
				backupIntervalVal = fmt.Sprintf("%dm", etcdBackupConfig.IntervalInMins)
			}

			intervalObjVal, convertDiags = resource_cluster.IntervalValue{
				BackupInterval:     getStrOrNullIfEmpty(backupIntervalVal),
				MaxBackupsToRetain: getIntOrNullIfZero(etcdBackupConfig.MaxIntervalBackupCount),
			}.ToObjectValue(ctx)
			diags.Append(convertDiags...)
			if diags.HasError() {
				return etcdBackupValue, diags
			}
		} else {
			intervalObjVal = types.ObjectNull(resource_cluster.IntervalValue{}.AttributeTypes(ctx))
		}
		var localPath string
		if etcdBackupConfig.StorageProperties.LocalPath != nil {
			localPath = *etcdBackupConfig.StorageProperties.LocalPath
		}

		etcdBackupObjVal, convertDiags := resource_cluster.EtcdBackupValue{
			StorageLocalPath: getStrOrNullIfEmpty(localPath),
			StorageType:      types.StringValue(etcdBackupConfig.StorageType),
			Daily:            dailyObjVal,
			Interval:         intervalObjVal,
		}.ToObjectValue(ctx)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return etcdBackupValue, diags
		}
		etcdBackupValue, convertDiags = resource_cluster.NewEtcdBackupValue(
			etcdBackupObjVal.AttributeTypes(ctx), etcdBackupObjVal.Attributes())
		diags.Append(convertDiags...)
		if diags.HasError() {
			return etcdBackupValue, diags
		}
		return etcdBackupValue, diags
	} else {
		return resource_cluster.NewEtcdBackupValueNull(), diags
	}
}

func getCustomRegistryValue(ctx context.Context, qbertCluster *qbert.Cluster) (resource_cluster.CustomRegistryValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if qbertCluster.CustomRegistryURL == "" {
		return resource_cluster.NewCustomRegistryValueNull(), diags
	}
	customRegistryObjValue, convertDiags := resource_cluster.CustomRegistryValue{
		Url:             getStrOrNullIfEmpty(qbertCluster.CustomRegistryURL),
		RepoPath:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryRepoPath),
		Username:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryUsername),
		Password:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryPassword),
		SkipTls:         getBoolFromIntPtr(qbertCluster.CustomRegistrySkipTLS),
		SelfSignedCerts: getBoolFromIntPtr(qbertCluster.CustomRegistrySelfSignedCerts),
		CertPath:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryCertPath),
	}.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.CustomRegistryValue{}, diags
	}
	customRegistryValue, convertDiags := resource_cluster.NewCustomRegistryValue(
		customRegistryObjValue.AttributeTypes(ctx), customRegistryObjValue.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.CustomRegistryValue{}, diags
	}
	return customRegistryValue, diags
}

func getEtcdValue(ctx context.Context, qbertCluster *qbert.Cluster) (resource_cluster.EtcdValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	etcdValue := resource_cluster.EtcdValue{
		DataDir:          getStrOrNullIfEmpty(qbertCluster.EtcdDataDir),
		Version:          getStrOrNullIfEmpty(qbertCluster.EtcdVersion),
		EnableEncryption: types.BoolValue(qbertCluster.EnableEtcdEncryption == "true"),
	}
	if qbertCluster.EtcdHeartbeatIntervalMs == "" {
		etcdValue.HeartbeatIntervalMs = types.Int64Value(0)
	} else {
		etcdHeartbeatIntervalMs, err := strconv.Atoi(qbertCluster.EtcdHeartbeatIntervalMs)
		if err != nil {
			diags.AddError("Failed to parse etcd heartbeat interval", err.Error())
			return etcdValue, diags
		}
		etcdValue.HeartbeatIntervalMs = types.Int64Value(int64(etcdHeartbeatIntervalMs))
	}
	if qbertCluster.EtcdElectionTimeoutMs == "" {
		etcdValue.ElectionTimeoutMs = types.Int64Value(0)
	} else {
		etcdElectionTimeoutMs, err := strconv.Atoi(qbertCluster.EtcdElectionTimeoutMs)
		if err != nil {
			diags.AddError("Failed to parse etcd election timeout", err.Error())
			return etcdValue, diags
		}
		etcdValue.ElectionTimeoutMs = types.Int64Value(int64(etcdElectionTimeoutMs))
	}
	etcdObjVal, convertDiags := etcdValue.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.EtcdValue{}, diags
	}
	etcdValue, convertDiags = resource_cluster.NewEtcdValue(etcdObjVal.AttributeTypes(ctx), etcdObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.EtcdValue{}, diags
	}
	return etcdValue, diags
}

func getCloudProviderValue(ctx context.Context, qbertCluster *qbert.Cluster) (resource_cluster.CloudProviderValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	cloudProviderObjValue, convertDiags := resource_cluster.CloudProviderValue{
		Uuid:              getStrOrNullIfEmpty(qbertCluster.CloudProviderUUID),
		Name:              getStrOrNullIfEmpty(qbertCluster.CloudProviderName),
		CloudProviderType: getStrOrNullIfEmpty(qbertCluster.CloudProviderType),
	}.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.CloudProviderValue{}, diags
	}
	cloudProviderValue, convertDiags := resource_cluster.NewCloudProviderValue(
		cloudProviderObjValue.AttributeTypes(ctx), cloudProviderObjValue.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.CloudProviderValue{}, diags
	}
	return cloudProviderValue, diags
}

func getK8sConfigValue(ctx context.Context, qbertCluster *qbert.Cluster) (resource_cluster.K8sConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if qbertCluster.RuntimeConfig == "" && qbertCluster.CloudProperties == nil {
		return resource_cluster.NewK8sConfigValueNull(), diags
	}
	if qbertCluster.CloudProperties == nil {
		k8sObjVal, convertDiags := resource_cluster.K8sConfigValue{
			ApiServerRuntimeConfig: getStrOrNullIfEmpty(qbertCluster.RuntimeConfig),
			ApiServerFlags:         types.ListNull(types.StringType),
			SchedulerFlags:         types.ListNull(types.StringType),
			ControllerManagerFlags: types.ListNull(types.StringType),
		}.ToObjectValue(ctx)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return resource_cluster.K8sConfigValue{}, diags
		}
		return resource_cluster.NewK8sConfigValue(k8sObjVal.AttributeTypes(ctx), k8sObjVal.Attributes())
	}

	apiServerFlagsList, convertDiags := strListFromJsonArr(ctx, qbertCluster.CloudProperties.APIServerFlags)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.K8sConfigValue{}, diags
	}
	schedulerFlagsList, convertDiags := strListFromJsonArr(ctx, qbertCluster.CloudProperties.SchedulerFlags)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.K8sConfigValue{}, diags
	}
	controllerManagerFlagsList, convertDiags := strListFromJsonArr(ctx, qbertCluster.CloudProperties.ControllerManagerFlags)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.K8sConfigValue{}, diags
	}
	k8sConfigValue := resource_cluster.K8sConfigValue{
		ApiServerRuntimeConfig: getStrOrNullIfEmpty(qbertCluster.RuntimeConfig),
		ApiServerFlags:         apiServerFlagsList,
		SchedulerFlags:         schedulerFlagsList,
		ControllerManagerFlags: controllerManagerFlagsList,
	}
	if k8sConfigValue.ApiServerRuntimeConfig.IsNull() && k8sConfigValue.ApiServerFlags.IsNull() &&
		k8sConfigValue.SchedulerFlags.IsNull() && k8sConfigValue.ControllerManagerFlags.IsNull() {
		return resource_cluster.NewK8sConfigValueNull(), diags
	}
	k8sConfigObjVal, convertDiags := k8sConfigValue.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.K8sConfigValue{}, diags
	}
	k8sConfigValue, convertDiags = resource_cluster.NewK8sConfigValue(k8sConfigObjVal.AttributeTypes(ctx), k8sConfigObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.K8sConfigValue{}, diags
	}
	return k8sConfigValue, diags
}

func getCalicoLimitsValue(ctx context.Context, qbertCluster *qbert.Cluster) (resource_cluster.CalicoLimitsValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	calicoLimitsValue := resource_cluster.CalicoLimitsValue{
		NodeCpuLimit:          getStrOrNullIfEmpty(qbertCluster.CalicoNodeCPULimit),
		NodeMemoryLimit:       getStrOrNullIfEmpty(qbertCluster.CalicoNodeMemoryLimit),
		TyphaCpuLimit:         getStrOrNullIfEmpty(qbertCluster.CalicoTyphaCPULimit),
		TyphaMemoryLimit:      getStrOrNullIfEmpty(qbertCluster.CalicoTyphaMemoryLimit),
		ControllerCpuLimit:    getStrOrNullIfEmpty(qbertCluster.CalicoControllerCPULimit),
		ControllerMemoryLimit: getStrOrNullIfEmpty(qbertCluster.CalicoControllerMemoryLimit),
	}
	calicoLimitsObjVal, convertDiags := calicoLimitsValue.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.CalicoLimitsValue{}, diags
	}
	calicoLimitsValue, convertDiags = resource_cluster.NewCalicoLimitsValue(calicoLimitsObjVal.AttributeTypes(ctx), calicoLimitsObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.CalicoLimitsValue{}, diags
	}
	return calicoLimitsValue, diags
}

func getStatusValue(ctx context.Context, qbertCluster *qbert.Cluster) (resource_cluster.StatusValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	statusValue := resource_cluster.StatusValue{
		Status:       getStrOrNullIfEmpty(qbertCluster.Status),
		LastOp:       getStrOrNullIfEmpty(qbertCluster.LastOp),
		LastOk:       getStrOrNullIfEmpty(qbertCluster.LastOk),
		TaskStatus:   getStrOrNullIfEmpty(qbertCluster.TaskStatus),
		TaskError:    getStrOrNullIfEmpty(qbertCluster.TaskError),
		MasterStatus: getStrOrNullIfEmpty(qbertCluster.MasterStatus),
		WorkerStatus: getStrOrNullIfEmpty(qbertCluster.WorkerStatus),
	}
	statusObjVal, convertDiags := statusValue.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.StatusValue{}, diags
	}
	statusValue, convertDiags = resource_cluster.NewStatusValue(statusObjVal.AttributeTypes(ctx), statusObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return resource_cluster.StatusValue{}, diags
	}
	return statusValue, diags
}
