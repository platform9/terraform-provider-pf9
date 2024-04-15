package provider

import (
	"context"
	"encoding/json"
	"net"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"k8s.io/utils/ptr"
)

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
func getIntOrNullIfZero(i int) types.Int64 {
	if i == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(i))
}

// qbert API returns empty string for null values, this function converts empty string to null to prevent
// Provider produced inconsistent result after apply, .external_dns_name: was null, but now cty.StringVal("")
func getStrOrNullIfEmpty(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func getBoolFromIntPtr(b *int) types.Bool {
	if b == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*b != 0)
}

func getIntPtrFromBool(b types.Bool) *int {
	if b.IsNull() || b.IsUnknown() {
		return nil
	}
	if b.ValueBool() {
		return ptr.To(1)
	}
	return ptr.To(0)
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

// CheckCIDROverlap checks if two CIDR blocks are overlapping
func CheckCIDROverlap(cidr1, cidr2 string) (bool, error) {
	_, network1, err := net.ParseCIDR(cidr1)
	if err != nil {
		return false, err
	}

	_, network2, err := net.ParseCIDR(cidr2)
	if err != nil {
		return false, err
	}
	return network1.Contains(network2.IP) || network2.Contains(network1.IP), nil
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

// Builds str list from json string array
func strListFromJsonArr(ctx context.Context, strArrayStr string) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if strArrayStr == "" {
		return types.ListNull(types.StringType), diags
	}
	var strSlice []string
	err := json.Unmarshal([]byte(strArrayStr), &strSlice)
	if err != nil {
		diags.AddError("Failed to parse string array", err.Error())
		return types.ListNull(types.StringType), diags
	}
	return types.ListValueFrom(ctx, types.StringType, strSlice)
}

func toJsonArrFromStrList(ctx context.Context, strList types.List) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if strList.IsNull() {
		return "", diags
	}
	strSlice := make([]string, len(strList.Elements()))
	convertDiags := strList.ElementsAs(ctx, &strSlice, false)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return "", diags
	}
	jsonBytes, err := json.Marshal(strSlice)
	if err != nil {
		diags.AddError("Failed to convert string array to json", err.Error())
		return "", diags
	}
	return string(jsonBytes), diags
}
