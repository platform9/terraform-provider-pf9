package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	"github.com/platform9/terraform-provider-pf9/internal/provider/resource_cluster"
)

func (r *clusterResource) reconcileCorednsAddon(ctx context.Context, clusterID string, coredns basetypes.ObjectValue, stateAddonsMap map[string]sunpikev1alpha2.ClusterAddon) diag.Diagnostics {
	var diags diag.Diagnostics
	var planVersion string
	var isVersionDifferent, isParamDifferent bool
	if coredns.IsNull() || coredns.IsUnknown() {
		return diags
	}
	objValueable, diags := resource_cluster.CorednsType{}.ValueFromObject(ctx, coredns)
	if diags.HasError() {
		return diags
	}
	planCoredns := objValueable.(resource_cluster.CorednsValue)
	if planCoredns.IsEnabled.ValueBool() {
		if _, found := stateAddonsMap["coredns"]; !found {
			tflog.Debug(ctx, "Enabling addon")
			err := r.enableAddon(ctx, AddonSpec{
				ClusterID: clusterID,
				// TODO: Use API to get the version
				// Version:   planCoredns.Version.ValueString(),
				Type:      "coredns",
				ParamsMap: map[string]string{},
			})
			if err != nil {
				diags.AddError("Failed to enable addon", err.Error())
				return diags
			}
		} else {
			tflog.Debug(ctx, "Checking if addon version and params needs to be patched")
			if !planCoredns.Version.IsNull() && !planCoredns.Version.IsUnknown() {
				// TODO: Define string addon names in const.go
				planVersion = planCoredns.Version.ValueString()
				stateVersion := stateAddonsMap["coredns"].Spec.Version
				if planVersion != stateVersion {
					isVersionDifferent = true
				}
			}
			planParams := map[string]string{}
			if !planCoredns.Params.IsNull() && !planCoredns.Params.IsUnknown() {
				diags.Append(planCoredns.Params.ElementsAs(ctx, &planParams, false)...)
				if diags.HasError() {
					// TODO: Decide whether to return or proceed when error with one attr
					return diags
				}
				stateParams := convertParamsToMap(stateAddonsMap["coredns"].Spec.Override.Params)
				if areMapsDifferent(planParams, stateParams) {
					isParamDifferent = true
				}
			}
			if isVersionDifferent || isParamDifferent {
				diags.Append(r.patchAddon(ctx, stateAddonsMap["coredns"], isVersionDifferent, isParamDifferent, planVersion, planParams)...)
				if diags.HasError() {
					return diags
				}
			}
		}
	} else {
		if _, found := stateAddonsMap["coredns"]; found {
			tflog.Debug(ctx, "Disabling addon")
			err := r.disableAddon(ctx, clusterID, "coredns")
			if err != nil {
				diags.AddError("Failed to disable addon", err.Error())
				return diags
			}
		} else {
			tflog.Debug(ctx, "Addon is already disabled")
		}
	}
	return diags
}

func (r *clusterResource) reconcileMonitoringAddon(ctx context.Context, clusterID string, addonName string, objValue basetypes.ObjectValue, stateAddonsMap map[string]sunpikev1alpha2.ClusterAddon) diag.Diagnostics {
	var diags diag.Diagnostics
	var planVersion string
	var isVersionDifferent, isParamDifferent bool
	if objValue.IsNull() || objValue.IsUnknown() {
		return diags
	}
	objValueable, diags := resource_cluster.MonitoringType{}.ValueFromObject(ctx, objValue)
	if diags.HasError() {
		return diags
	}
	planAddon := objValueable.(resource_cluster.MonitoringValue)
	if planAddon.IsEnabled.ValueBool() {
		if _, found := stateAddonsMap[addonName]; !found {
			tflog.Debug(ctx, "Enabling addon")
			err := r.enableAddon(ctx, AddonSpec{
				ClusterID: clusterID,
				// TODO: Use API to get the version
				// Version:   planAddon.Version.ValueString(),
				Type:      addonName,
				ParamsMap: map[string]string{},
			})
			if err != nil {
				diags.AddError("Failed to enable addon", err.Error())
				return diags
			}
		} else {
			tflog.Debug(ctx, "Checking if addon version and params needs to be patched")
			if !planAddon.Version.IsNull() && !planAddon.Version.IsUnknown() {
				// TODO: Define string addon names in const.go
				planVersion = planAddon.Version.ValueString()
				stateVersion := stateAddonsMap[addonName].Spec.Version
				if planVersion != stateVersion {
					isVersionDifferent = true
				}
			}
			planParams := map[string]string{}
			if !planAddon.Params.IsNull() && !planAddon.Params.IsUnknown() {
				diags.Append(planAddon.Params.ElementsAs(ctx, &planParams, false)...)
				if diags.HasError() {
					// TODO: Decide whether to return or proceed when error with one attr
					return diags
				}
				stateParams := convertParamsToMap(stateAddonsMap[addonName].Spec.Override.Params)
				if areMapsDifferent(planParams, stateParams) {
					isParamDifferent = true
				}
			}
			if isVersionDifferent || isParamDifferent {
				diags.Append(r.patchAddon(ctx, stateAddonsMap[addonName], isVersionDifferent, isParamDifferent, planVersion, planParams)...)
				if diags.HasError() {
					return diags
				}
			}
		}
	} else {
		if _, found := stateAddonsMap[addonName]; found {
			tflog.Debug(ctx, "Disabling addon")
			err := r.disableAddon(ctx, clusterID, addonName)
			if err != nil {
				diags.AddError("Failed to disable addon", err.Error())
				return diags
			}
		} else {
			tflog.Debug(ctx, "Addon is already disabled")
		}
	}
	return diags
}

func (r *clusterResource) reconcileMetallbAddon(ctx context.Context, clusterID string, addonName string, objValue basetypes.ObjectValue, stateAddonsMap map[string]sunpikev1alpha2.ClusterAddon) diag.Diagnostics {
	var diags diag.Diagnostics
	var planVersion string
	var isVersionDifferent, isParamDifferent bool
	if objValue.IsNull() || objValue.IsUnknown() {
		return diags
	}
	objValueable, diags := resource_cluster.MetallbType{}.ValueFromObject(ctx, objValue)
	if diags.HasError() {
		return diags
	}
	planAddon := objValueable.(resource_cluster.MetallbValue)
	if planAddon.IsEnabled.ValueBool() {
		if _, found := stateAddonsMap[addonName]; !found {
			tflog.Debug(ctx, "Enabling addon")
			err := r.enableAddon(ctx, AddonSpec{
				ClusterID: clusterID,
				// TODO: Use API to get the version
				// Version:   planAddon.Version.ValueString(),
				Type:      addonName,
				ParamsMap: map[string]string{},
			})
			if err != nil {
				diags.AddError("Failed to enable addon", err.Error())
				return diags
			}
		} else {
			tflog.Debug(ctx, "Checking if addon version and params needs to be patched")
			if !planAddon.Version.IsNull() && !planAddon.Version.IsUnknown() {
				// TODO: Define string addon names in const.go
				planVersion = planAddon.Version.ValueString()
				stateVersion := stateAddonsMap[addonName].Spec.Version
				if planVersion != stateVersion {
					isVersionDifferent = true
				}
			}
			planParams := map[string]string{}
			if !planAddon.Params.IsNull() && !planAddon.Params.IsUnknown() {
				diags.Append(planAddon.Params.ElementsAs(ctx, &planParams, false)...)
				if diags.HasError() {
					// TODO: Decide whether to return or proceed when error with one attr
					return diags
				}
				stateParams := convertParamsToMap(stateAddonsMap[addonName].Spec.Override.Params)
				if areMapsDifferent(planParams, stateParams) {
					isParamDifferent = true
				}
			}
			if isVersionDifferent || isParamDifferent {
				diags.Append(r.patchAddon(ctx, stateAddonsMap[addonName], isVersionDifferent, isParamDifferent, planVersion, planParams)...)
				if diags.HasError() {
					return diags
				}
			}
		}
	} else {
		if _, found := stateAddonsMap[addonName]; found {
			tflog.Debug(ctx, "Disabling addon")
			err := r.disableAddon(ctx, clusterID, addonName)
			if err != nil {
				diags.AddError("Failed to disable addon", err.Error())
				return diags
			}
		} else {
			tflog.Debug(ctx, "Addon is already disabled")
		}
	}
	return diags
}
