package resource_cluster

import "github.com/hashicorp/terraform-plugin-framework/attr"

func NewKnownAddonsValueMust(addonsValue AddonsValue) AddonsValue {
	if addonsValue.Name.IsUnknown() {
		panic("Name attribute is unknown")
	}
	if addonsValue.Enabled.IsUnknown() {
		panic("Enabled attribute is unknown")
	}
	if addonsValue.Version.IsUnknown() {
		panic("Version attribute is unknown")
	}
	if addonsValue.Config.IsUnknown() {
		panic("Config attribute is unknown")
	}
	addonsValue.state = attr.ValueStateKnown
	return addonsValue
}
