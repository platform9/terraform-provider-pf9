package resource_cluster

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// DefaultAddons returns the default addons for a cluster.
// Add the following to the provider_code_spec.json:
//
//	"addons": {
//	  "map_nested": {
//	    "default": {
//		     "custom": {
//			   "schema_definition": "DefaultAddons()"
//		     }
//	    }
//	  }
//	}
//
// FIXME: This does not work as intended hence currenty unused.
func DefaultAddons() defaults.Map {
	ctx := context.Background()
	addonsMapValue, diags := GetDefaultAddons(ctx)
	if diags.HasError() {
		panic("Failed to create default addons map value")
	}
	return mapdefault.StaticValue(addonsMapValue)
}

func GetDefaultAddons(ctx context.Context) (basetypes.MapValue, diag.Diagnostics) {
	// TODO: Investigate why the default addon parameters are not being preserved in the plan.
	// During observation, the parameter values are set correctly until the `ModifyPlan()` function is called,
	// but they are lost after that. The `Create()` function then receives all the other fields except the params.
	// This needs to be investigated to understand why the parameter values are not being carried through to the final plan.
	addonMap := map[string]AddonsValue{}
	addonMap["coredns"] = getAddonWithParams(ctx, map[string]string{
		"dnsMemoryLimit": "170Mi",
		"dnsDomain":      "cluster.local",
	})
	addonMap["kubernetes-dashboard"] = getAddonWithParams(ctx, map[string]string{})
	addonMap["metrics-server"] = getAddonWithParams(ctx, map[string]string{
		"metricsMemoryLimit": "300Mi",
		"metricsCpuLimit":    "100m",
	})
	addonMap["monitoring"] = getAddonWithParams(ctx, map[string]string{
		"retentionTime": "7d",
	})
	return types.MapValueFrom(ctx, AddonsValue{}.Type(ctx), addonMap)
}

func getAddonWithParams(ctx context.Context, params map[string]string) AddonsValue {
	defaultParamsMapValue, diags := types.MapValueFrom(ctx, types.StringType, params)
	if diags.HasError() {
		panic("Failed to parse default addons params")
	}
	objValue, diags := AddonsValue{
		Params: defaultParamsMapValue,
	}.ToObjectValue(ctx)
	if diags.HasError() {
		panic("Failed to create default addon object value")
	}
	objValuable, diags := AddonsType{}.ValueFromObject(ctx, objValue)
	if diags.HasError() {
		panic("Failed to create default addons value from object value")
	}
	return objValuable.(AddonsValue)
}
