package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_cluster"
	"github.com/platform9/terraform-provider-pf9/internal/provider/resource_cluster"
)

var _ datasource.DataSource = (*clusterDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*clusterDataSource)(nil)

func NewClusterDataSource() datasource.DataSource {
	return &clusterDataSource{}
}

type clusterDataSource struct {
	client *pmk.HTTPClient
}

func (d *clusterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (d *clusterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_cluster.ClusterDataSourceSchema(ctx)
}

func (d *clusterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*pmk.HTTPClient)
}

func (d *clusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_cluster.ClusterModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	clusterID := data.Id.ValueString()
	// TODO: Frequent Auth() call pollutes the code, rewrite some parts
	// of pmk httpclient to simplify use of it
	authInfo, err := d.client.Authenticator().Auth(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	projectID := authInfo.ProjectID
	tflog.Info(ctx, "Reading cluster from qbert", map[string]interface{}{"clusterID": clusterID})
	cluster, err := d.client.Qbert().GetCluster(ctx, projectID, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster", err.Error())
		return
	}
	resp.Diagnostics.Append(qbertClusterToDatasource(ctx, cluster, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "Listing nodes attached to the cluster", map[string]interface{}{"clusterID": clusterID})
	clusterNodes, err := d.client.Qbert().ListClusterNodes(ctx, clusterID)
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
	var diags diag.Diagnostics
	data.MasterNodes, diags = types.SetValueFrom(ctx, basetypes.StringType{}, masterNodes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.WorkerNodes, diags = types.SetValueFrom(ctx, basetypes.StringType{}, workerNodes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "Listing addons enabled on the cluster", map[string]interface{}{"clusterID": clusterID})
	qbertAddons, err := d.client.Qbert().ListClusterAddons(fmt.Sprintf("sunpike.pf9.io/cluster=%s", clusterID))
	if err != nil {
		resp.Diagnostics.AddError("Failed to get cluster addons", err.Error())
		return
	}
	addonsValue, diags := sunpikeAddonsToDatasource(ctx, qbertAddons.Items)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	data.Addons, diags = types.MapValueFrom(ctx, resource_cluster.AddonsValue{}.Type(ctx), addonsValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func sunpikeAddonsToDatasource(ctx context.Context, sunpikeAddons []sunpikev1alpha2.ClusterAddon) (map[string]datasource_cluster.AddonsValue, diag.Diagnostics) {
	tfAddonsMap := map[string]datasource_cluster.AddonsValue{}
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
		addonObjVal, diags := datasource_cluster.AddonsValue{
			Version: version,
			Phase:   phase,
			Params:  params,
		}.ToObjectValue(ctx)
		if diags.HasError() {
			return tfAddonsMap, diags
		}
		addonObjValuable, diags := datasource_cluster.AddonsType{}.ValueFromObject(ctx, addonObjVal)
		if diags.HasError() {
			return tfAddonsMap, diags
		}
		tfAddonsMap[sunpikeAddon.Spec.Type] = addonObjValuable.(datasource_cluster.AddonsValue)
	}
	return tfAddonsMap, diags
}

func qbertClusterToDatasource(ctx context.Context, qbertCluster *qbert.Cluster, clusterModel *datasource_cluster.ClusterModel) diag.Diagnostics {
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
	clusterModel.KubeRoleVersion = types.StringValue(qbertCluster.KubeRoleVersion)
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
		etcdBackupObjVal, convertDiags := datasource_cluster.EtcdBackupValue{
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
		etcdBackup, convertDiags := datasource_cluster.NewEtcdBackupValue(
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
		tfMap, d := types.MapValueFrom(ctx, types.StringType, tagsGoMap)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		clusterModel.Tags = tfMap
	}

	return diags
}
