package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *clusterResource) readAddonsFromRemote(ctx context.Context, clusterID string) ([]sunpikev1alpha2.ClusterAddon, error) {
	tflog.Info(ctx, "Listing addons enabled on the cluster", map[string]interface{}{"clusterID": clusterID})
	qbertAddons, err := r.client.Qbert().ListClusterAddons(fmt.Sprintf("sunpike.pf9.io/cluster=%s", clusterID))
	if err != nil {
		tflog.Error(ctx, "Failed to get cluster addons", map[string]interface{}{"error": err})
		return nil, err
	}
	return qbertAddons.Items, nil
}

type AddonSpec struct {
	ClusterID string
	Version   string
	Type      string
	ParamsMap map[string]string
}

func (r *clusterResource) enableAddon(ctx context.Context, spec AddonSpec) error {
	// UI sends the following POST
	// {
	// 	"kind": "ClusterAddon",
	// 	"apiVersion": "sunpike.platform9.com/v1alpha2",
	// 	"metadata": {
	// 		"name": "791d2744-f23f-4170-bd85-37d5b771b7bd-luigi",
	// 		"labels": {
	// 			"sunpike.pf9.io/cluster": "791d2744-f23f-4170-bd85-37d5b771b7bd",
	// 			"type": "luigi"
	// 		}
	// 	},
	// 	"spec": {
	// 		"clusterID": "791d2744-f23f-4170-bd85-37d5b771b7bd",
	// 		"version": "0.5.4",
	// 		"type": "luigi",
	// 		"override": {},
	// 		"watch": true
	// 	}
	// }
	tflog.Debug(ctx, "Enabling addon", map[string]interface{}{"addon": spec.Type})
	var params []sunpikev1alpha2.Params
	for key, value := range spec.ParamsMap {
		params = append(params, sunpikev1alpha2.Params{
			Name:  key,
			Value: value,
		})
	}
	return r.client.Sunpike().Create(ctx, &sunpikev1alpha2.ClusterAddon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", spec.ClusterID, spec.Type),
			Namespace: "default",
			Labels: map[string]string{
				"sunpike.pf9.io/cluster": spec.ClusterID,
				"type":                   spec.Type,
			},
		},
		Spec: sunpikev1alpha2.ClusterAddonSpec{
			Type:    spec.Type,
			Version: spec.Version,
			Override: sunpikev1alpha2.Override{
				Params: params,
			},
		},
	})
}

type StrMap map[string]string

func (p StrMap) Equals(other StrMap) bool {
	if len(p) != len(other) {
		return false
	}
	for key, value := range p {
		if otherValue, found := other[key]; !found || otherValue != value {
			return false
		}
	}
	for key, value := range other {
		if pValue, found := (p)[key]; !found || pValue != value {
			return false
		}
	}
	return true
}

// patchAddon patches Addon using sunpike API, patch includes changing overrides and version
func (r *clusterResource) patchAddon(ctx context.Context, stateAddon *sunpikev1alpha2.ClusterAddon,
	planAddonSpec AddonSpec) diag.Diagnostics {
	var diags diag.Diagnostics
	var isParamPatchNeeded bool
	stateParamsMap := convertParamsToMap(stateAddon.Spec.Override.Params)
	if !StrMap(planAddonSpec.ParamsMap).Equals(StrMap(stateParamsMap)) {
		tflog.Debug(ctx, "Override params do not match with the plan, patch required",
			map[string]interface{}{
				"tfAddon":       planAddonSpec.Type,
				"sunpikeAddon":  stateAddon.Spec.Type,
				"tfParams":      planAddonSpec.ParamsMap,
				"sunpikeParams": stateParamsMap,
			})
		isParamPatchNeeded = true
	}
	if !isParamPatchNeeded && stateAddon.Spec.Version == planAddonSpec.Version {
		tflog.Debug(ctx, "Addon is already in desired state", map[string]interface{}{"addon": stateAddon.Spec.Type})
		return diags
	}

	tflog.Debug(ctx, "Patching addon", map[string]interface{}{"addon": stateAddon.Spec.Type})
	patch := client.MergeFrom(stateAddon.DeepCopy())
	if isParamPatchNeeded {
		var planAddonParams []sunpikev1alpha2.Params
		for key, value := range planAddonSpec.ParamsMap {
			planAddonParams = append(planAddonParams, sunpikev1alpha2.Params{
				Name:  key,
				Value: value,
			})
		}
		stateAddon.Spec.Override.Params = planAddonParams
	}
	// Version is always required for patching; whether it changed or not
	stateAddon.Spec.Version = planAddonSpec.Version
	err := r.client.Sunpike().Patch(ctx, stateAddon, patch)
	if err != nil {
		tflog.Error(ctx, "Failed to patch addon", map[string]interface{}{"error": err})
		diags.AddError("Failed to patch addon", err.Error())
		return diags
	}
	return diags
}

func convertParamsToMap(params []sunpikev1alpha2.Params) map[string]string {
	paramsMap := map[string]string{}
	for _, param := range params {
		paramsMap[param.Name] = param.Value
	}
	return paramsMap
}

func (r *clusterResource) listAddonsByLabels(ctx context.Context, clusterID string, addonType string) ([]sunpikev1alpha2.ClusterAddon, error) {
	labelSelector := labels.SelectorFromSet(map[string]string{
		"sunpike.pf9.io/cluster": clusterID,
		"type":                   addonType,
	})

	listOptions := &client.ListOptions{
		Namespace:     "default",
		LabelSelector: labelSelector,
	}

	var clusterAddonsList sunpikev1alpha2.ClusterAddonList
	err := r.client.Sunpike().List(ctx, &clusterAddonsList, listOptions)
	if err != nil {
		return nil, err
	}
	return clusterAddonsList.Items, nil
}

func (r *clusterResource) disableAddon(ctx context.Context, clusterID string, addonType string) error {
	clusterAddons, err := r.listAddonsByLabels(ctx, clusterID, addonType)
	if err != nil {
		return err
	}
	if len(clusterAddons) == 0 {
		tflog.Debug(ctx, "Addon is already disabled")
		return nil
	}
	clusterAddon := clusterAddons[0]
	return r.client.Sunpike().Delete(ctx, &clusterAddon)
}
