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
	var data resource_cluster.ClusterModel

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

	// Cluster does not exist yet
	createClusterReq, d := r.CreateCreateClusterRequest(ctx, authInfo.ProjectID, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to create create cluster object")
		return
	}
	krVersions, err := r.client.Qbert().ListSupportedVersions(authInfo.ProjectID)
	if err != nil {
		tflog.Error(ctx, "Failed to get supported versions", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get supported versions", err.Error())
		return
	}
	if createClusterReq.KubeRoleVersion == "" {
		tflog.Debug(ctx, "Supported versions", map[string]interface{}{"krVersions": krVersions})
		if len(krVersions.Roles) > 0 {
			latestKubeRoleVersion := findLatestKubeRoleVersion(krVersions.Roles)
			createClusterReq.KubeRoleVersion = latestKubeRoleVersion.RoleVersion
		}
	} else {
		tflog.Debug(ctx, "KubeRoleVersion provided", map[string]interface{}{"kubeRoleVersion": createClusterReq.KubeRoleVersion})
		allowedKubeRoleVersions := []string{}
		for _, role := range krVersions.Roles {
			allowedKubeRoleVersions = append(allowedKubeRoleVersions, role.RoleVersion)
		}
		if !StrSliceContains(allowedKubeRoleVersions, createClusterReq.KubeRoleVersion) {
			resp.Diagnostics.AddError("KubeRoleVersion provided is not supported", fmt.Sprintf("Supported versions: %v", allowedKubeRoleVersions))
			return
		}
	}

	reqBody, err := json.Marshal(createClusterReq)
	if err != nil {
		tflog.Error(ctx, "Failed to marshal create cluster request", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to marshal create cluster request", err.Error())
		return
	}
	tflog.Debug(ctx, "Creating a cluster: %v", map[string]interface{}{"createClusterReq": string(reqBody)})
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
	tflog.Debug(ctx, "Attaching nodes", map[string]interface{}{"nodeList": nodeList})
	err = qbertClient.AttachNodes(clusterID, nodeList)
	if err != nil {
		tflog.Error(ctx, "Failed to attach nodes", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to attach nodes", err.Error())
		return
	}
	tflog.Debug(ctx, "Attached nodes, saving state", map[string]interface{}{"nodeList": nodeList})
	data.Id = types.StringValue(clusterID)
	cluster, err := qbertClient.GetCluster(ctx, projectID, clusterID)
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError("Failed to get cluster", err.Error())
		return
	}
	diags := qbertClusterToTerraformCluster(ctx, cluster, &data)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	clusterNodes, err := r.client.Qbert().ListClusterNodes(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster nodes", err.Error())
		return
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
	var convertDiags diag.Diagnostics
	data.MasterNodes, convertDiags = types.SetValueFrom(ctx, basetypes.StringType{}, masterNodes)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.WorkerNodes, convertDiags = types.SetValueFrom(ctx, basetypes.StringType{}, workerNodes)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data resource_cluster.ClusterModel

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
	cluster, err := r.client.Qbert().GetCluster(ctx, projectID, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster", err.Error())
		return
	}
	tflog.Info(ctx, "Cluster: %v", map[string]interface{}{"cluster": cluster})
	diags := qbertClusterToTerraformCluster(ctx, cluster, &data)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	clusterNodes, err := r.client.Qbert().ListClusterNodes(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster nodes", err.Error())
		return
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
	var convertDiags diag.Diagnostics
	data.MasterNodes, convertDiags = types.SetValueFrom(ctx, basetypes.StringType{}, masterNodes)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.WorkerNodes, convertDiags = types.SetValueFrom(ctx, basetypes.StringType{}, workerNodes)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
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

	if !plan.CertExpiryHrs.Equal(state.CertExpiryHrs) {
		editRequired = true
		editClusterReq.CertExpiryHrs = ptr.To(int(plan.CertExpiryHrs.ValueInt64()))
	}
	if !plan.ExternalDnsName.Equal(state.ExternalDnsName) {
		editRequired = true
		editClusterReq.ExternalDnsName = plan.ExternalDnsName.ValueStringPointer()
	}
	if !plan.MasterIp.Equal(state.MasterIp) {
		editRequired = true
		editClusterReq.MasterIp = plan.MasterIp.ValueStringPointer()
	}
	if !plan.EnableMetallb.Equal(state.EnableMetallb) {
		editRequired = true
		editClusterReq.EnableMetalLb = plan.EnableMetallb.ValueBoolPointer()
	}
	if !plan.MetallbCidr.Equal(state.MetallbCidr) {
		editClusterReq.MetallbCidr = plan.MetallbCidr.ValueStringPointer()
	}
	if !plan.CalicoNodeCpuLimit.Equal(state.CalicoNodeCpuLimit) {
		editRequired = true
		editClusterReq.CalicoNodeCpuLimit = plan.CalicoNodeCpuLimit.ValueString()
	}
	if !plan.CalicoNodeMemoryLimit.Equal(state.CalicoNodeMemoryLimit) {
		editRequired = true
		editClusterReq.CalicoNodeMemoryLimit = plan.CalicoNodeMemoryLimit.ValueString()
	}
	if !plan.CalicoTyphaCpuLimit.Equal(state.CalicoTyphaCpuLimit) {
		editRequired = true
		editClusterReq.CalicoTyphaCpuLimit = plan.CalicoTyphaCpuLimit.ValueString()
	}
	if !plan.CalicoTyphaMemoryLimit.Equal(state.CalicoTyphaMemoryLimit) {
		editRequired = true
		editClusterReq.CalicoTyphaMemoryLimit = plan.CalicoTyphaMemoryLimit.ValueString()
	}
	if !plan.CalicoControllerCpuLimit.Equal(state.CalicoControllerCpuLimit) {
		editRequired = true
		editClusterReq.CalicoControllerCpuLimit = plan.CalicoControllerCpuLimit.ValueString()
	}
	if !plan.CalicoControllerMemoryLimit.Equal(state.CalicoControllerMemoryLimit) {
		editRequired = true
		editClusterReq.CalicoControllerMemoryLimit = plan.CalicoControllerMemoryLimit.ValueString()
	}
	if !plan.ContainersCidr.Equal(state.ContainersCidr) {
		editRequired = true
		editClusterReq.ContainersCidr = plan.ContainersCidr.ValueStringPointer()
	}
	if !plan.ServicesCidr.Equal(state.ServicesCidr) {
		editRequired = true
		editClusterReq.ServicesCidr = plan.ServicesCidr.ValueStringPointer()
	}

	// TODO: Add following fields to provider_code_spec.json if they are required
	// editClusterReq.KubeProxyMode = plan.KubeProxyMode.ValueStringPointer()
	// editClusterReq.EnableProfileAgent = plan.EnableProfileAgent.ValueBoolPointer()
	// editClusterReq.EnableCatapultMonitoring = plan.EnableCatapultMonitoring.ValueBoolPointer()
	// editClusterReq.NumMaxWorkers = plan.NumMaxWorkers.ValueInt64Pointer()
	// editClusterReq.NumMinWorkers = plan.NumMinWorkers.ValueInt64Pointer()
	// editClusterReq.NumWorkers = plan.NumWorkers.ValueInt64Pointer()

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

	cluster, err := r.client.Qbert().GetCluster(ctx, projectID, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster", err.Error())
		return
	}

	diags := qbertClusterToTerraformCluster(ctx, cluster, &state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// TODO: Use addon api to get addon related attributes
	// /qbert/v4/projectID/sunpike/apis/sunpike.platform9.com/v1alpha2/namespaces/default/clusteraddons?labelSelector=sunpike.pf9.io%2Fcluster%3D<clusterID>

	clusterNodes, err := r.client.Qbert().ListClusterNodes(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster nodes", err.Error())
		return
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
	var convertDiags diag.Diagnostics
	state.MasterNodes, convertDiags = types.SetValueFrom(ctx, basetypes.StringType{}, masterNodes)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.WorkerNodes, convertDiags = types.SetValueFrom(ctx, basetypes.StringType{}, workerNodes)
	resp.Diagnostics.Append(convertDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

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
	out.DeployLuigiOperator = types.BoolValue(in.DeployLuigiOperator != 0)
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
	out.EnableMetallb = types.BoolValue(in.EnableMetallb != 0)
	out.MetallbCidr = types.StringValue(in.MetallbCidr)

	// Computed attributes
	out.CanUpgrade = types.BoolValue(in.CanUpgrade)
	out.CreatedAt = types.StringValue(in.CreatedAt)
	out.IsKubernetes = types.BoolValue(in.IsKubernetes != 0)
	out.IsMesos = types.BoolValue(in.IsMesos != 0)
	out.IsSwarm = types.BoolValue(in.IsSwarm != 0)
	out.Debug = types.BoolValue(in.Debug == "true")
	out.Status = types.StringValue(in.Status)
	out.FlannelIfaceLabel = types.StringValue(in.FlannelIfaceLabel)
	out.FlannelPublicIfaceLabel = types.StringValue(in.FlannelPublicIfaceLabel)
	out.DockerRoot = types.StringValue(in.DockerRoot)
	out.EtcdDataDir = types.StringValue(in.EtcdDataDir)
	out.LastOp = types.StringValue(in.LastOp)
	out.LastOk = types.StringValue(in.LastOk)
	out.KeystoneEnabled = types.BoolValue(in.KeystoneEnabled != 0)
	out.AuthzEnabled = types.BoolValue(in.AuthzEnabled != 0)
	out.TaskStatus = types.StringValue(in.TaskStatus)
	out.TaskError = types.StringValue(in.TaskError)
	out.KubeProxyMode = types.StringValue(in.KubeProxyMode)
	out.NumMasters = types.Int64Value(int64(in.NumMasters))
	out.NumWorkers = types.Int64Value(int64(in.NumWorkers))
	out.AppCatalogEnabled = types.BoolValue(in.AppCatalogEnabled != 0)
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
	out.ApiServerStorageBackend = types.StringValue(in.ApiserverStorageBackend)
	out.EnableCas = types.BoolValue(in.EnableCAS != 0)
	out.NumMinWorkers = types.Int64Value(int64(in.NumMinWorkers))
	out.NumMaxWorkers = types.Int64Value(int64(in.NumMaxWorkers))
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
	out.CanMinorUpgrade = types.BoolValue(in.CanMinorUpgrade != 0)
	out.CanPatchUpgrade = types.BoolValue(in.CanPatchUpgrade != 0)
	out.PatchUpgradeRoleVersion = types.StringValue(in.PatchUpgradeRoleVersion)
	out.NodePoolName = types.StringValue(in.NodePoolName)
	out.CloudProviderUuid = types.StringValue(in.CloudProviderUuid)
	out.CloudProviderName = types.StringValue(in.CloudProviderName)
	out.CloudProviderType = types.StringValue(in.CloudProviderType)
	out.DeployKubevirt = types.BoolValue(in.DeployKubevirt != 0)
	out.UpgradingTo = types.StringValue(in.UpgradingTo)
	out.ReservedCpus = types.StringValue(in.ReservedCPUs)
	out.DockerPrivateRegistry = types.StringValue(in.DockerPrivateRegistry)
	out.QuayPrivateRegistry = types.StringValue(in.QuayPrivateRegistry)
	out.GcrPrivateRegistry = types.StringValue(in.GcrPrivateRegistry)
	out.K8sPrivateRegistry = types.StringValue(in.K8sPrivateRegistry)
	out.EnableCatapultMonitoring = types.BoolValue(in.EnableCatapultMonitoring)
	out.DockerCentosPackageRepoUrl = types.StringValue(in.DockerCentosPackageRepoUrl)
	out.DockerUbuntuPackageRepoUrl = types.StringValue(in.DockerUbuntuPackageRepoUrl)
	out.AddonOperatorImageTag = types.StringValue(in.AddonOperatorImageTag)
	out.IsAirGapped = types.BoolValue(in.IsAirgapped != 0)
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
	req.DeployLuigiOperator = in.DeployLuigiOperator.ValueBoolPointer()
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

	// If user has not included the attribute then it should not be sent to the API
	if in.EnableMetallb.IsNull() {
		req.EnableMetalLb = in.EnableMetallb.ValueBoolPointer()
	}
	req.MetallbCidr = in.MetallbCidr.ValueString()

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
