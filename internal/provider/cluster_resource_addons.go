package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	sunpikev1alpha2 "github.com/platform9/pf9-sdk-go/pf9/apis/sunpike/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AddonSpec struct {
	ClusterID string
	Version   string
	Type      string
	ParamsMap map[string]string
}

type AddonsClient interface {
	Get(ctx context.Context, addonName string) (sunpikev1alpha2.ClusterAddon, error)
	List(ctx context.Context, clusterID string, addonType string) ([]sunpikev1alpha2.ClusterAddon, error)
	Enable(ctx context.Context, addonSpec AddonSpec) error
	Disable(ctx context.Context, addonSpec AddonSpec) error
	Patch(ctx context.Context, addonSpec AddonSpec, refClusterAddon *sunpikev1alpha2.ClusterAddon) error
}

type addonsClient struct {
	client client.Client
}

func NewAddonClient(sunpikeClient client.Client) AddonsClient {
	return &addonsClient{client: sunpikeClient}
}

func (r *addonsClient) Enable(ctx context.Context, spec AddonSpec) error {
	var params []sunpikev1alpha2.Params
	for key, value := range spec.ParamsMap {
		params = append(params, sunpikev1alpha2.Params{
			Name:  key,
			Value: value,
		})
	}
	return r.client.Create(ctx, &sunpikev1alpha2.ClusterAddon{
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

// Patch patches Addon using sunpike API, patch includes changing overrides and version
func (r *addonsClient) Patch(ctx context.Context,
	planAddonSpec AddonSpec, stateAddon *sunpikev1alpha2.ClusterAddon) error {
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
		return nil
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
	err := r.client.Patch(ctx, stateAddon, patch)
	if err != nil {
		return fmt.Errorf("failed to patch addon: %w", err)
	}
	return nil
}

func convertParamsToMap(params []sunpikev1alpha2.Params) map[string]string {
	paramsMap := map[string]string{}
	for _, param := range params {
		paramsMap[param.Name] = param.Value
	}
	return paramsMap
}

func (r *addonsClient) Get(ctx context.Context, addonName string) (sunpikev1alpha2.ClusterAddon, error) {
	var sunpikeAddon sunpikev1alpha2.ClusterAddon
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      addonName,
		Namespace: "default",
	}, &sunpikeAddon)
	return sunpikeAddon, err
}

func (r *addonsClient) List(ctx context.Context, clusterID string, addonType string) ([]sunpikev1alpha2.ClusterAddon, error) {
	labelSelector := labels.SelectorFromSet(map[string]string{
		"sunpike.pf9.io/cluster": clusterID,
		"type":                   addonType,
	})
	listOptions := &client.ListOptions{
		Namespace:     "default",
		LabelSelector: labelSelector,
	}
	var clusterAddonsList sunpikev1alpha2.ClusterAddonList
	err := r.client.List(ctx, &clusterAddonsList, listOptions)
	if err != nil {
		return nil, err
	}
	return clusterAddonsList.Items, nil
}

func (r *addonsClient) Disable(ctx context.Context, addonSpec AddonSpec) error {
	// TODO: Use r.client.Get() assuming name = clusterID-addonType
	clusterAddons, err := r.List(ctx, addonSpec.ClusterID, addonSpec.Type)
	if err != nil {
		return err
	}
	if len(clusterAddons) == 0 {
		tflog.Debug(ctx, "Addon is already disabled")
		return nil
	}
	clusterAddon := clusterAddons[0]
	return r.client.Delete(ctx, &clusterAddon)
}

var addonAliases = map[string]string{
	"kubevirt":             "kubevirtaddon",
	"profile-agent":        "pf9-profile-agent",
	"kubernetes-dashboard": "dashboard",
}

func getDefaultAddonVersion(defaults map[string]string, addonType string) string {
	// APIs have different names for the same addon
	if alias, ok := addonAliases[addonType]; ok {
		addonType = alias
	}
	if version, ok := defaults[addonType]; ok {
		return version
	}
	return ""
}
