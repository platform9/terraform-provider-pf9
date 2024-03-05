package provider

import (
	"context"
	"encoding/json"
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
	tflog.Info(ctx, "AuthInfo: %v", map[string]interface{}{"authInfo": authInfo})
	projectID := authInfo.ProjectID

	// Cluster does not exist yet
	createClusterReq, d := r.CreateCreateClusterRequest(ctx, authInfo.ProjectID, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "Failed to create create cluster object")
		return
	}
	if createClusterReq.KubeRoleVersion == "" {
		krVersions, err := r.client.Qbert().ListSupportedVersions(authInfo.ProjectID)
		if err != nil {
			tflog.Error(ctx, "Failed to get supported versions", map[string]interface{}{"error": err})
			resp.Diagnostics.AddError("Failed to get supported versions", err.Error())
			return
		}
		tflog.Debug(ctx, "Supported versions", map[string]interface{}{"krVersions": krVersions})
		if len(krVersions.Roles) > 0 {
			// TODO: How to identify the default role version? How UI does it?
			createClusterReq.KubeRoleVersion = krVersions.Roles[len(krVersions.Roles)-1].RoleVersion
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
	if !plan.Name.Equal(state.Name) {
		editClusterReq.Name = plan.Name.ValueStringPointer()
	}

	var editRequired bool
	if !plan.EtcdBackup.Equal(state.EtcdBackup) {
		editRequired = true
		var etcdConfig qbert.EtcdBackupConfig
		etcdConfig.DailyBackupTime = plan.EtcdBackup.DailyBackupTime.ValueStringPointer()
		if plan.EtcdBackup.IsEtcdBackupEnabled.ValueBool() {
			etcdConfig.IsEtcdBackupEnabled = ptr.To(1)
		} else {
			etcdConfig.IsEtcdBackupEnabled = ptr.To(0)
		}
		etcdConfig.MaxTimestampBackupCount = ptr.To(int(plan.EtcdBackup.MaxTimestampBackupCount.ValueInt64()))
		etcdConfig.StorageProperties.LocalPath = plan.EtcdBackup.StorageLocalPath.ValueStringPointer()
		etcdConfig.StorageType = plan.EtcdBackup.StorageType.ValueStringPointer()
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

	// TODO: Add following fields to provider_code_spec.json
	// editClusterReq.KubeProxyMode = plan.KubeProxyMode.ValueStringPointer()
	// editClusterReq.EnableProfileAgent = plan.EnableProfileAgent.ValueBoolPointer()
	// editClusterReq.EnableCatapultMonitoring = plan.EnableCatapultMonitoring.ValueBoolPointer()
	// editClusterReq.NumMaxWorkers = plan.NumMaxWorkers.ValueInt64Pointer()
	// editClusterReq.NumMinWorkers = plan.NumMinWorkers.ValueInt64Pointer()
	// editClusterReq.NumWorkers = plan.NumWorkers.ValueInt64Pointer()

	if !plan.Tags.Equal(state.Tags) {
		editRequired = true
		editClusterReq.Tags = map[string]string{}
		tagsGoMap := map[string]string{}
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tagsGoMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		editClusterReq.Tags = tagsGoMap
	}
	if editRequired {
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
	tflog.Info(ctx, "AuthInfo: %v", map[string]interface{}{"authInfo": authInfo})
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
	if in.CloudProperties != nil {
		if len(in.CloudProperties.MasterNodes) > 0 {
			masterNodes := []string{}
			err := json.Unmarshal([]byte(in.CloudProperties.MasterNodes), &masterNodes)
			if err != nil {
				tflog.Error(ctx, "Failed to unmarshal master nodes", map[string]interface{}{"error": err})
				diags.AddError("Failed to unmarshal master nodes", err.Error())
				return diags
			}
			setVal, d := types.SetValueFrom(ctx, basetypes.StringType{}, masterNodes)

			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			out.MasterNodes = setVal
		}
		if len(in.CloudProperties.WorkerNodes) > 0 {
			workerNodes := []string{}
			err := json.Unmarshal([]byte(in.CloudProperties.WorkerNodes), &workerNodes)
			if err != nil {
				tflog.Error(ctx, "Failed to unmarshal worker nodes", map[string]interface{}{"error": err})
				diags.AddError("Failed to unmarshal worker nodes", err.Error())
				return diags
			}
			setVal, d := types.SetValueFrom(ctx, basetypes.StringType{}, workerNodes)
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			out.WorkerNodes = setVal
		}
		if len(in.CloudProperties.Monitoring) > 0 {
			var monitoring qbert.MonitoringConfig
			err := json.Unmarshal([]byte(in.CloudProperties.Monitoring), &monitoring)
			if err != nil {
				tflog.Error(ctx, "Failed to unmarshal monitoring", map[string]interface{}{"error": err})
				diags.AddError("Failed to unmarshal monitoring", err.Error())
				return diags
			}

			if monitoring.RetentionTime != nil {
				mVal, d := resource_cluster.NewMonitoringValue(
					resource_cluster.MonitoringValue{}.AttributeTypes(ctx),
					map[string]attr.Value{
						"retention_time": basetypes.NewStringValue(*monitoring.RetentionTime),
					},
				)
				diags = append(diags, d...)
				if diags.HasError() {
					tflog.Error(ctx, "Failed to create monitoring value", map[string]interface{}{"error": d})
					diags.AddError("Failed to create monitoring value", "Failed to create monitoring value")
					return diags
				}
				out.Monitoring = mVal
			} else {
				out.Monitoring = resource_cluster.NewMonitoringValueNull()
			}
		}
	}
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
				"is_etcd_backup_enabled":     types.BoolValue(*in.EtcdBackup.IsEtcdBackupEnabled != 0),
				"storage_type":               types.StringValue(*in.EtcdBackup.StorageType),
				"max_timestamp_backup_count": types.Int64Value(int64(*in.EtcdBackup.MaxTimestampBackupCount)),
				"storage_local_path":         localPathVal,
				"daily_backup_time":          types.StringValue(*in.EtcdBackup.DailyBackupTime),
			},
		)
		if d.HasError() {
			return d
		}
		out.EtcdBackup = etcdBackup
	}
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
		//TODO: Validate worker nodes and master nodes are mutually exclusive
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
	req.InterfaceDetectionMethod = in.InterfaceDetectionMethod.ValueString()
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
	req.EtcdBackup.DailyBackupTime = in.EtcdBackup.DailyBackupTime.ValueStringPointer()
	if in.EtcdBackup.IsEtcdBackupEnabled.ValueBool() {
		req.EtcdBackup.IsEtcdBackupEnabled = ptr.To(1)
	} else {
		req.EtcdBackup.IsEtcdBackupEnabled = ptr.To(0)
	}
	req.EtcdBackup.MaxTimestampBackupCount = ptr.To(int(in.EtcdBackup.MaxTimestampBackupCount.ValueInt64()))
	req.EtcdBackup.StorageProperties.LocalPath = in.EtcdBackup.StorageLocalPath.ValueStringPointer()
	req.EtcdBackup.StorageType = in.EtcdBackup.StorageType.ValueStringPointer()
	req.Monitoring.RetentionTime = in.Monitoring.RetentionTime.ValueStringPointer()

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
		// Privileged:                ptr.To(true),
		// ContainerRuntime:          qbert.ContainerRuntimeContainerd,
		// ContainerCIDR:             "10.20.0.0/16",
		// ServiceCIDR:               "10.21.0.0/16",
		// CalicoIPIPMode:            "Always",
		// MTUSize:                   ptr.To(1440),
		// CalicoNatOutgoing:         ptr.To(true),
		// CalicoV4BlockSize:         "24",
		// CalicoIpv4DetectionMethod: "can-reach=8.8.8.8",
		// DeployLuigiOperator:       ptr.To(false), // We deploy a custom version of Luigi
		// // EnableCAS:                 ptr.To(false),
		// // EnableEtcdEncryption: "true", // Dont know why this is string type in API
		EtcdBackup: &qbert.EtcdBackupConfig{
			// IsEtcdBackupEnabled: ptr.To(1),
			// StorageType:         ptr.To("local"),
			// StorageProperties: qbert.StorageProperties{
			// 	LocalPath: ptr.To("/etc/pf9/etcd-backup"),
			// },
		},
		Monitoring: &qbert.MonitoringConfig{
			// RetentionTime: ptr.To("7d"),
		},
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
