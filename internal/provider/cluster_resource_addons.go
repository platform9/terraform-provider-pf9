package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	"github.com/platform9/terraform-provider-pf9/internal/provider/resource_cluster"
)

func (r *clusterResource) reconcileCorednsAddon(ctx context.Context, clusterID string, addonName string, planAddon basetypes.ObjectValue, stateAddon basetypes.ObjectValue, stateAddonSunpike *sunpikev1alpha2.ClusterAddon, defaultAddonVersion string, isCreate bool) diag.Diagnostics {
	var diags diag.Diagnostics
	var planVersion string
	var isVersionDifferent, areParamsDifferent bool
	if planAddon.Equal(stateAddon) {
		tflog.Debug(ctx, "Addon is in sync")
		return diags
	}
	if planAddon.IsUnknown() {
		tflog.Debug(ctx, "Addon is unknown in plan")
		// TODO: When does this happen? Should we return an error?
		return diags
	}
	if planAddon.IsNull() && !stateAddon.IsNull() {
		if isCreate {
			// default enabled addons after create are not disabled
			return diags
		}
		tflog.Debug(ctx, "Disable addon because plan is null")
		err := r.disableAddon(ctx, clusterID, "coredns")
		if err != nil {
			diags.AddError("Failed to disable addon", err.Error())
		}
		return diags
	}
	if !planAddon.IsNull() {
		var version string
		objValueable, diags := resource_cluster.CorednsType{}.ValueFromObject(ctx, planAddon)
		if diags.HasError() {
			return diags
		}
		planAddonValue := objValueable.(resource_cluster.CorednsValue)
		if !planAddonValue.Version.IsNull() && !planAddonValue.Version.IsUnknown() {
			version = planAddonValue.Version.ValueString()
		} else {
			version = defaultAddonVersion
		}
		planParamsMap := map[string]string{}
		if !planAddonValue.Params.IsNull() && !planAddonValue.Params.IsUnknown() {
			diags.Append(planAddonValue.Params.ElementsAs(ctx, &planParamsMap, false)...)
			if diags.HasError() {
				return diags
			}
		}
		if stateAddon.IsNull() {
			tflog.Debug(ctx, "Enable addon because state is null")
			err := r.enableAddon(ctx, AddonSpec{
				ClusterID: clusterID,
				Version:   version,
				Type:      addonName,
				ParamsMap: planParamsMap,
			})
			if err != nil {
				diags.AddError("Failed to enable addon", err.Error())
			}
			return diags
		}
		tflog.Debug(ctx, "Checking if addon version and params needs to be patched")
		objValueable, diags = resource_cluster.CorednsType{}.ValueFromObject(ctx, stateAddon)
		if diags.HasError() {
			return diags
		}
		stateParamsMap := map[string]string{}
		stateAddonValue := objValueable.(resource_cluster.CorednsValue)
		if !planAddonValue.Params.Equal(stateAddonValue.Params) {
			diags.Append(stateAddonValue.Params.ElementsAs(ctx, &stateParamsMap, false)...)
			if diags.HasError() {
				return diags
			}
			if areMapsDifferent(planParamsMap, stateParamsMap) {
				// Copy non overlapping params from state to plan
				for key, val := range stateParamsMap {
					if _, found := planParamsMap[key]; !found {
						planParamsMap[key] = val
					}
				}
				areParamsDifferent = true
				tflog.Debug(ctx, "Params are different as per go compare", map[string]interface{}{"plan": planParamsMap, "state": stateParamsMap})
			}
		}

		if planAddonValue.Version.IsNull() || planAddonValue.Version.IsUnknown() {
			planVersion = defaultAddonVersion
		} else {
			planVersion = planAddonValue.Version.ValueString()
		}
		if !planAddonValue.Version.Equal(stateAddonValue.Version) {
			stateVersion := stateAddonValue.Version.ValueString()
			if planVersion != stateVersion {
				tflog.Debug(ctx, "Version is different", map[string]interface{}{"plan": planVersion, "state": stateVersion})
			}
			isVersionDifferent = true
		}
		if isVersionDifferent || areParamsDifferent {
			diags.Append(r.patchAddon(ctx, *stateAddonSunpike, areParamsDifferent, planVersion, planParamsMap)...)
			if diags.HasError() {
				return diags
			}
		}
	}
	return diags
}
