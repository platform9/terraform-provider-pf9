package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	"github.com/platform9/pf9-sdk-go/pf9/pmk"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"github.com/platform9/terraform-provider-pf9/internal/provider/datasource_cluster"
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
	// TODO: This function is copied from the cluster_resource and import changed from resource_cluster to datasource_cluster
	// Need to refactor this code to avoid duplication. Check if there's a way to reuse the code from resource for ds
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
	data.MasterNodes, diags = types.SetValueFrom(ctx, types.StringType, masterNodes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.WorkerNodes, diags = types.SetValueFrom(ctx, types.StringType, workerNodes)
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
	data.Addons, diags = types.MapValueFrom(ctx, datasource_cluster.AddonsValue{}.Type(ctx), addonsValue)
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
		var params types.Map
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
	// KubeRoleVersion does not change immediately after cluster upgrade
	// hence this is a workaround to get the correct value
	if qbertCluster.UpgradingTo != "" {
		clusterModel.KubeRoleVersion = types.StringValue(qbertCluster.UpgradingTo)
	} else {
		clusterModel.KubeRoleVersion = types.StringValue(qbertCluster.KubeRoleVersion)
	}
	if qbertCluster.K8sApiPort == "" {
		clusterModel.K8sApiPort = types.Int64Null()
	} else {
		intPort, err := strconv.Atoi(qbertCluster.K8sApiPort)
		if err != nil {
			diags.AddError("Failed to parse k8s api port", err.Error())
			return diags
		}
		clusterModel.K8sApiPort = types.Int64Value(int64(intPort))
	}
	clusterModel.CpuManagerPolicy = types.StringValue(qbertCluster.CPUManagerPolicy)
	clusterModel.TopologyManagerPolicy = types.StringValue(qbertCluster.TopologyManagerPolicy)
	clusterModel.ReservedCpus = getStrOrNullIfEmpty(qbertCluster.ReservedCPUs)
	clusterModel.CalicoIpIpMode = types.StringValue(qbertCluster.CalicoIpIpMode)
	clusterModel.CalicoNatOutgoing = types.BoolValue(qbertCluster.CalicoNatOutgoing != 0)
	clusterModel.CalicoV4BlockSize = types.StringValue(qbertCluster.CalicoV4BlockSize)
	clusterModel.CalicoIpv4DetectionMethod = types.StringValue(qbertCluster.CalicoIPv4DetectionMethod)
	clusterModel.NetworkPlugin = types.StringValue(qbertCluster.NetworkPlugin)
	clusterModel.ContainerRuntime = types.StringValue(qbertCluster.ContainerRuntime)
	if qbertCluster.EnableCatapultMonitoring != nil {
		clusterModel.EnableCatapultMonitoring = types.BoolValue(*qbertCluster.EnableCatapultMonitoring)
	}
	k8sconfig, convertDiags := getDSK8sConfigValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.K8sConfig = k8sconfig

	clusterModel.ExternalDnsName = getStrOrNullIfEmpty(qbertCluster.ExternalDnsName)
	clusterModel.CertExpiryHrs = types.Int64Value(int64(qbertCluster.CertExpiryHrs))
	calicoLimits, convertDiags := getDSCalicoLimitsValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.CalicoLimits = calicoLimits

	// Computed attributes
	statusValue, convertDiags := getDSStatusValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.Status = statusValue

	clusterModel.FlannelIfaceLabel = getStrOrNullIfEmpty(qbertCluster.FlannelIfaceLabel)
	clusterModel.FlannelPublicIfaceLabel = getStrOrNullIfEmpty(qbertCluster.FlannelPublicIfaceLabel)
	clusterModel.DockerRoot = types.StringValue(qbertCluster.DockerRoot)
	clusterModel.MasterVipVrouterId = types.StringValue(qbertCluster.MasterVipVrouterId)

	clusterModel.ProjectId = types.StringValue(qbertCluster.ProjectId)
	clusterModel.CreatedAt = types.StringValue(qbertCluster.CreatedAt)
	clusterModel.CalicoIpv4 = types.StringValue(qbertCluster.CalicoIPv4)
	clusterModel.CalicoIpv6 = types.StringValue(qbertCluster.CalicoIPv6)
	clusterModel.CalicoIpv6DetectionMethod = types.StringValue(qbertCluster.CalicoIPv6DetectionMethod)
	clusterModel.CalicoRouterId = types.StringValue(qbertCluster.CalicoRouterID)
	clusterModel.CalicoIpv6PoolCidr = getStrOrNullIfEmpty(qbertCluster.CalicoIPv6PoolCidr)
	clusterModel.CalicoIpv6PoolBlockSize = types.StringValue(qbertCluster.CalicoIPv6PoolBlockSize)
	clusterModel.CalicoIpv6PoolNatOutgoing = types.BoolValue(qbertCluster.CalicoIPv6PoolNatOutgoing != 0)
	clusterModel.FelixIpv6Support = types.BoolValue(qbertCluster.FelixIPv6Support != 0)
	clusterModel.Masterless = types.BoolValue(qbertCluster.Masterless != 0)

	clusterModel.Ipv6 = types.BoolValue(qbertCluster.IPv6 != 0)
	clusterModel.NodePoolName = types.StringValue(qbertCluster.NodePoolName)
	cloudProviderValue, convertDiags := getDSCloudProviderValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.CloudProvider = cloudProviderValue
	clusterModel.DockerPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.DockerPrivateRegistry)
	clusterModel.QuayPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.QuayPrivateRegistry)
	clusterModel.GcrPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.GcrPrivateRegistry)
	clusterModel.K8sPrivateRegistry = getStrOrNullIfEmpty(qbertCluster.K8sPrivateRegistry)
	clusterModel.DockerCentosPackageRepoUrl = getStrOrNullIfEmpty(qbertCluster.DockerCentosPackageRepoUrl)
	clusterModel.DockerUbuntuPackageRepoUrl = getStrOrNullIfEmpty(qbertCluster.DockerUbuntuPackageRepoUrl)
	clusterModel.InterfaceReachableIp = types.StringValue(qbertCluster.InterfaceReachableIP)
	customRegistry, convertDiags := getDSCustomRegistryValue(ctx, qbertCluster)
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

	etcdValue, convertDiags := getDSEtcdValue(ctx, qbertCluster)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return diags
	}
	clusterModel.Etcd = etcdValue
	etcdBackupValue, convertDiags := getDSEtcdBackupValue(ctx, qbertCluster.EtcdBackup)
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

func getDSEtcdBackupValue(ctx context.Context, etcdBackupConfig *qbert.EtcdBackupConfig) (datasource_cluster.EtcdBackupValue, diag.Diagnostics) {
	etcdBackupValue := datasource_cluster.EtcdBackupValue{}
	var diags diag.Diagnostics
	if etcdBackupConfig != nil && etcdBackupConfig.IsEtcdBackupEnabled == 1 {
		var dailyObjVal, intervalObjVal types.Object
		var convertDiags diag.Diagnostics
		if etcdBackupConfig.DailyBackupTime != "" {
			dailyObjVal, convertDiags = datasource_cluster.DailyValue{
				BackupTime:         getStrOrNullIfEmpty(etcdBackupConfig.DailyBackupTime),
				MaxBackupsToRetain: getIntOrNullIfZero(etcdBackupConfig.MaxTimestampBackupCount),
			}.ToObjectValue(ctx)
			diags.Append(convertDiags...)
			if diags.HasError() {
				return etcdBackupValue, diags
			}
		} else {
			dailyObjVal = types.ObjectNull(datasource_cluster.DailyValue{}.AttributeTypes(ctx))
		}

		if etcdBackupConfig.IntervalInHours != 0 || etcdBackupConfig.IntervalInMins != 0 {
			var backupIntervalVal string
			if etcdBackupConfig.IntervalInHours != 0 {
				backupIntervalVal = fmt.Sprintf("%dh", etcdBackupConfig.IntervalInHours)
			} else if etcdBackupConfig.IntervalInMins != 0 {
				backupIntervalVal = fmt.Sprintf("%dm", etcdBackupConfig.IntervalInMins)
			}

			intervalObjVal, convertDiags = datasource_cluster.IntervalValue{
				BackupInterval:     getStrOrNullIfEmpty(backupIntervalVal),
				MaxBackupsToRetain: getIntOrNullIfZero(etcdBackupConfig.MaxIntervalBackupCount),
			}.ToObjectValue(ctx)
			diags.Append(convertDiags...)
			if diags.HasError() {
				return etcdBackupValue, diags
			}
		} else {
			intervalObjVal = types.ObjectNull(datasource_cluster.IntervalValue{}.AttributeTypes(ctx))
		}
		var localPath string
		if etcdBackupConfig.StorageProperties.LocalPath != nil {
			localPath = *etcdBackupConfig.StorageProperties.LocalPath
		}

		etcdBackupObjVal, convertDiags := datasource_cluster.EtcdBackupValue{
			StorageLocalPath: getStrOrNullIfEmpty(localPath),
			StorageType:      types.StringValue(etcdBackupConfig.StorageType),
			Daily:            dailyObjVal,
			Interval:         intervalObjVal,
		}.ToObjectValue(ctx)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return etcdBackupValue, diags
		}
		etcdBackupValue, convertDiags = datasource_cluster.NewEtcdBackupValue(
			etcdBackupObjVal.AttributeTypes(ctx), etcdBackupObjVal.Attributes())
		diags.Append(convertDiags...)
		if diags.HasError() {
			return etcdBackupValue, diags
		}
		return etcdBackupValue, diags
	} else {
		return datasource_cluster.NewEtcdBackupValueNull(), diags
	}
}

func getDSCustomRegistryValue(ctx context.Context, qbertCluster *qbert.Cluster) (datasource_cluster.CustomRegistryValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if qbertCluster.CustomRegistryUrl == "" {
		return datasource_cluster.NewCustomRegistryValueNull(), diags
	}
	customRegistryObjValue, convertDiags := datasource_cluster.CustomRegistryValue{
		Url:             getStrOrNullIfEmpty(qbertCluster.CustomRegistryUrl),
		RepoPath:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryRepoPath),
		Username:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryUsername),
		Password:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryPassword),
		SkipTls:         getBoolFromIntPtr(qbertCluster.CustomRegistrySkipTls),
		SelfSignedCerts: getBoolFromIntPtr(qbertCluster.CustomRegistrySelfSignedCerts),
		CertPath:        getStrOrNullIfEmpty(qbertCluster.CustomRegistryCertPath),
	}.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.CustomRegistryValue{}, diags
	}
	customRegistryValue, convertDiags := datasource_cluster.NewCustomRegistryValue(
		customRegistryObjValue.AttributeTypes(ctx), customRegistryObjValue.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.CustomRegistryValue{}, diags
	}
	return customRegistryValue, diags
}

func getDSEtcdValue(ctx context.Context, qbertCluster *qbert.Cluster) (datasource_cluster.EtcdValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	etcdValue := datasource_cluster.EtcdValue{
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
		return datasource_cluster.EtcdValue{}, diags
	}
	etcdValue, convertDiags = datasource_cluster.NewEtcdValue(etcdObjVal.AttributeTypes(ctx), etcdObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.EtcdValue{}, diags
	}
	return etcdValue, diags
}

func getDSCloudProviderValue(ctx context.Context, qbertCluster *qbert.Cluster) (datasource_cluster.CloudProviderValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	cloudProviderObjValue, convertDiags := datasource_cluster.CloudProviderValue{
		Uuid:              getStrOrNullIfEmpty(qbertCluster.CloudProviderUuid),
		Name:              getStrOrNullIfEmpty(qbertCluster.CloudProviderName),
		CloudProviderType: getStrOrNullIfEmpty(qbertCluster.CloudProviderType),
	}.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.CloudProviderValue{}, diags
	}
	cloudProviderValue, convertDiags := datasource_cluster.NewCloudProviderValue(
		cloudProviderObjValue.AttributeTypes(ctx), cloudProviderObjValue.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.CloudProviderValue{}, diags
	}
	return cloudProviderValue, diags
}

func getDSK8sConfigValue(ctx context.Context, qbertCluster *qbert.Cluster) (datasource_cluster.K8sConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if qbertCluster.RuntimeConfig == "" && qbertCluster.CloudProperties == nil {
		return datasource_cluster.NewK8sConfigValueNull(), diags
	}
	if qbertCluster.CloudProperties == nil {
		k8sObjVal, convertDiags := datasource_cluster.K8sConfigValue{
			ApiServerRuntimeConfig: getStrOrNullIfEmpty(qbertCluster.RuntimeConfig),
			ApiServerFlags:         types.ListNull(types.StringType),
			SchedulerFlags:         types.ListNull(types.StringType),
			ControllerManagerFlags: types.ListNull(types.StringType),
		}.ToObjectValue(ctx)
		diags.Append(convertDiags...)
		if diags.HasError() {
			return datasource_cluster.K8sConfigValue{}, diags
		}
		return datasource_cluster.NewK8sConfigValue(k8sObjVal.AttributeTypes(ctx), k8sObjVal.Attributes())
	}

	apiServerFlagsList, convertDiags := strListFromJsonArr(ctx, qbertCluster.CloudProperties.ApiServerFlags)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.K8sConfigValue{}, diags
	}
	schedulerFlagsList, convertDiags := strListFromJsonArr(ctx, qbertCluster.CloudProperties.SchedulerFlags)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.K8sConfigValue{}, diags
	}
	controllerManagerFlagsList, convertDiags := strListFromJsonArr(ctx, qbertCluster.CloudProperties.ControllerManagerFlags)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.K8sConfigValue{}, diags
	}
	k8sConfigValue := datasource_cluster.K8sConfigValue{
		ApiServerRuntimeConfig: getStrOrNullIfEmpty(qbertCluster.RuntimeConfig),
		ApiServerFlags:         apiServerFlagsList,
		SchedulerFlags:         schedulerFlagsList,
		ControllerManagerFlags: controllerManagerFlagsList,
	}
	if k8sConfigValue.ApiServerRuntimeConfig.IsNull() && k8sConfigValue.ApiServerFlags.IsNull() &&
		k8sConfigValue.SchedulerFlags.IsNull() && k8sConfigValue.ControllerManagerFlags.IsNull() {
		return datasource_cluster.NewK8sConfigValueNull(), diags
	}
	k8sConfigObjVal, convertDiags := k8sConfigValue.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.K8sConfigValue{}, diags
	}
	k8sConfigValue, convertDiags = datasource_cluster.NewK8sConfigValue(k8sConfigObjVal.AttributeTypes(ctx), k8sConfigObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.K8sConfigValue{}, diags
	}
	return k8sConfigValue, diags
}

func getDSCalicoLimitsValue(ctx context.Context, qbertCluster *qbert.Cluster) (datasource_cluster.CalicoLimitsValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	calicoLimitsValue := datasource_cluster.CalicoLimitsValue{
		NodeCpuLimit:          getStrOrNullIfEmpty(qbertCluster.CalicoNodeCpuLimit),
		NodeMemoryLimit:       getStrOrNullIfEmpty(qbertCluster.CalicoNodeMemoryLimit),
		TyphaCpuLimit:         getStrOrNullIfEmpty(qbertCluster.CalicoTyphaCpuLimit),
		TyphaMemoryLimit:      getStrOrNullIfEmpty(qbertCluster.CalicoTyphaMemoryLimit),
		ControllerCpuLimit:    getStrOrNullIfEmpty(qbertCluster.CalicoControllerCpuLimit),
		ControllerMemoryLimit: getStrOrNullIfEmpty(qbertCluster.CalicoControllerMemoryLimit),
	}
	calicoLimitsObjVal, convertDiags := calicoLimitsValue.ToObjectValue(ctx)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.CalicoLimitsValue{}, diags
	}
	calicoLimitsValue, convertDiags = datasource_cluster.NewCalicoLimitsValue(calicoLimitsObjVal.AttributeTypes(ctx), calicoLimitsObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.CalicoLimitsValue{}, diags
	}
	return calicoLimitsValue, diags
}

func getDSStatusValue(ctx context.Context, qbertCluster *qbert.Cluster) (datasource_cluster.StatusValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	statusValue := datasource_cluster.StatusValue{
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
		return datasource_cluster.StatusValue{}, diags
	}
	statusValue, convertDiags = datasource_cluster.NewStatusValue(statusObjVal.AttributeTypes(ctx), statusObjVal.Attributes())
	diags.Append(convertDiags...)
	if diags.HasError() {
		return datasource_cluster.StatusValue{}, diags
	}
	return statusValue, diags
}
