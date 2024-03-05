// Code generated by terraform-plugin-framework-generator DO NOT EDIT.

package datasource_nodes

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func NodesDataSourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"filter": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required:            true,
						Description:         "Name of the attribute on which this filter is applied",
						MarkdownDescription: "Name of the attribute on which this filter is applied",
						Validators: []validator.String{
							stringvalidator.OneOf("id", "name", "api_responding", "cluster_name", "cluster_uuid", "is_master", "node_pool_name", "node_pool_uuid", "primary_ip", "status"),
						},
					},
					"values": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
					},
				},
				CustomType: FilterType{
					ObjectType: types.ObjectType{
						AttrTypes: FilterValue{}.AttributeTypes(ctx),
					},
				},
				Required: true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Placeholder for ID",
				MarkdownDescription: "Placeholder for ID",
			},
			"nodes": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"api_responding": schema.BoolAttribute{
							Computed:            true,
							Description:         "Indicates the API server on this node is running",
							MarkdownDescription: "Indicates the API server on this node is running",
						},
						"cluster_name": schema.StringAttribute{
							Computed:            true,
							Description:         "Name of the cluster the node belongs to",
							MarkdownDescription: "Name of the cluster the node belongs to",
						},
						"cluster_uuid": schema.StringAttribute{
							Computed:            true,
							Description:         "UUID of the cluster the node belongs to",
							MarkdownDescription: "UUID of the cluster the node belongs to",
						},
						"id": schema.StringAttribute{
							Computed:            true,
							Description:         "UUID of the cluster",
							MarkdownDescription: "UUID of the cluster",
						},
						"is_master": schema.BoolAttribute{
							Computed:            true,
							Description:         "true if this node is a master of a cluster.",
							MarkdownDescription: "true if this node is a master of a cluster.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							Description:         "Host name of the node",
							MarkdownDescription: "Host name of the node",
						},
						"node_pool_name": schema.StringAttribute{
							Computed:            true,
							Description:         "Name of the node pool, the node belongs to",
							MarkdownDescription: "Name of the node pool, the node belongs to",
						},
						"node_pool_uuid": schema.StringAttribute{
							Computed:            true,
							Description:         "UUID of the node pool, the node belongs to",
							MarkdownDescription: "UUID of the node pool, the node belongs to",
						},
						"primary_ip": schema.StringAttribute{
							Computed:            true,
							Description:         "IP address of the node",
							MarkdownDescription: "IP address of the node",
						},
						"status": schema.StringAttribute{
							Computed:            true,
							Description:         "Status of the node. States include ok, converging, failed. These states indicate the current state of kubernetes setup on the host.",
							MarkdownDescription: "Status of the node. States include ok, converging, failed. These states indicate the current state of kubernetes setup on the host.",
						},
					},
					CustomType: NodesType{
						ObjectType: types.ObjectType{
							AttrTypes: NodesValue{}.AttributeTypes(ctx),
						},
					},
				},
				Computed:            true,
				Description:         "List of nodes",
				MarkdownDescription: "List of nodes",
			},
		},
	}
}

type NodesModel struct {
	Filter FilterValue  `tfsdk:"filter"`
	Id     types.String `tfsdk:"id"`
	Nodes  types.List   `tfsdk:"nodes"`
}

var _ basetypes.ObjectTypable = FilterType{}

type FilterType struct {
	basetypes.ObjectType
}

func (t FilterType) Equal(o attr.Type) bool {
	other, ok := o.(FilterType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t FilterType) String() string {
	return "FilterType"
}

func (t FilterType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return nil, diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	valuesAttribute, ok := attributes["values"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`values is missing from object`)

		return nil, diags
	}

	valuesVal, ok := valuesAttribute.(basetypes.ListValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`values expected to be basetypes.ListValue, was: %T`, valuesAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return FilterValue{
		Name:   nameVal,
		Values: valuesVal,
		state:  attr.ValueStateKnown,
	}, diags
}

func NewFilterValueNull() FilterValue {
	return FilterValue{
		state: attr.ValueStateNull,
	}
}

func NewFilterValueUnknown() FilterValue {
	return FilterValue{
		state: attr.ValueStateUnknown,
	}
}

func NewFilterValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (FilterValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing FilterValue Attribute Value",
				"While creating a FilterValue value, a missing attribute value was detected. "+
					"A FilterValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FilterValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid FilterValue Attribute Type",
				"While creating a FilterValue value, an invalid attribute value was detected. "+
					"A FilterValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FilterValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("FilterValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra FilterValue Attribute Value",
				"While creating a FilterValue value, an extra attribute value was detected. "+
					"A FilterValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra FilterValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewFilterValueUnknown(), diags
	}

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return NewFilterValueUnknown(), diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	valuesAttribute, ok := attributes["values"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`values is missing from object`)

		return NewFilterValueUnknown(), diags
	}

	valuesVal, ok := valuesAttribute.(basetypes.ListValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`values expected to be basetypes.ListValue, was: %T`, valuesAttribute))
	}

	if diags.HasError() {
		return NewFilterValueUnknown(), diags
	}

	return FilterValue{
		Name:   nameVal,
		Values: valuesVal,
		state:  attr.ValueStateKnown,
	}, diags
}

func NewFilterValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) FilterValue {
	object, diags := NewFilterValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewFilterValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t FilterType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewFilterValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewFilterValueUnknown(), nil
	}

	if in.IsNull() {
		return NewFilterValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewFilterValueMust(FilterValue{}.AttributeTypes(ctx), attributes), nil
}

func (t FilterType) ValueType(ctx context.Context) attr.Value {
	return FilterValue{}
}

var _ basetypes.ObjectValuable = FilterValue{}

type FilterValue struct {
	Name   basetypes.StringValue `tfsdk:"name"`
	Values basetypes.ListValue   `tfsdk:"values"`
	state  attr.ValueState
}

func (v FilterValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 2)

	var val tftypes.Value
	var err error

	attrTypes["name"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["values"] = basetypes.ListType{
		ElemType: types.StringType,
	}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 2)

		val, err = v.Name.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["name"] = val

		val, err = v.Values.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["values"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v FilterValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v FilterValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v FilterValue) String() string {
	return "FilterValue"
}

func (v FilterValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	valuesVal, d := types.ListValue(types.StringType, v.Values.Elements())

	diags.Append(d...)

	if d.HasError() {
		return types.ObjectUnknown(map[string]attr.Type{
			"name": basetypes.StringType{},
			"values": basetypes.ListType{
				ElemType: types.StringType,
			},
		}), diags
	}

	objVal, diags := types.ObjectValue(
		map[string]attr.Type{
			"name": basetypes.StringType{},
			"values": basetypes.ListType{
				ElemType: types.StringType,
			},
		},
		map[string]attr.Value{
			"name":   v.Name,
			"values": valuesVal,
		})

	return objVal, diags
}

func (v FilterValue) Equal(o attr.Value) bool {
	other, ok := o.(FilterValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.Name.Equal(other.Name) {
		return false
	}

	if !v.Values.Equal(other.Values) {
		return false
	}

	return true
}

func (v FilterValue) Type(ctx context.Context) attr.Type {
	return FilterType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v FilterValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"name": basetypes.StringType{},
		"values": basetypes.ListType{
			ElemType: types.StringType,
		},
	}
}

var _ basetypes.ObjectTypable = NodesType{}

type NodesType struct {
	basetypes.ObjectType
}

func (t NodesType) Equal(o attr.Type) bool {
	other, ok := o.(NodesType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t NodesType) String() string {
	return "NodesType"
}

func (t NodesType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	apiRespondingAttribute, ok := attributes["api_responding"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`api_responding is missing from object`)

		return nil, diags
	}

	apiRespondingVal, ok := apiRespondingAttribute.(basetypes.BoolValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`api_responding expected to be basetypes.BoolValue, was: %T`, apiRespondingAttribute))
	}

	clusterNameAttribute, ok := attributes["cluster_name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`cluster_name is missing from object`)

		return nil, diags
	}

	clusterNameVal, ok := clusterNameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`cluster_name expected to be basetypes.StringValue, was: %T`, clusterNameAttribute))
	}

	clusterUuidAttribute, ok := attributes["cluster_uuid"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`cluster_uuid is missing from object`)

		return nil, diags
	}

	clusterUuidVal, ok := clusterUuidAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`cluster_uuid expected to be basetypes.StringValue, was: %T`, clusterUuidAttribute))
	}

	idAttribute, ok := attributes["id"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`id is missing from object`)

		return nil, diags
	}

	idVal, ok := idAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`id expected to be basetypes.StringValue, was: %T`, idAttribute))
	}

	isMasterAttribute, ok := attributes["is_master"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`is_master is missing from object`)

		return nil, diags
	}

	isMasterVal, ok := isMasterAttribute.(basetypes.BoolValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`is_master expected to be basetypes.BoolValue, was: %T`, isMasterAttribute))
	}

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return nil, diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	nodePoolNameAttribute, ok := attributes["node_pool_name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`node_pool_name is missing from object`)

		return nil, diags
	}

	nodePoolNameVal, ok := nodePoolNameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`node_pool_name expected to be basetypes.StringValue, was: %T`, nodePoolNameAttribute))
	}

	nodePoolUuidAttribute, ok := attributes["node_pool_uuid"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`node_pool_uuid is missing from object`)

		return nil, diags
	}

	nodePoolUuidVal, ok := nodePoolUuidAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`node_pool_uuid expected to be basetypes.StringValue, was: %T`, nodePoolUuidAttribute))
	}

	primaryIpAttribute, ok := attributes["primary_ip"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`primary_ip is missing from object`)

		return nil, diags
	}

	primaryIpVal, ok := primaryIpAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`primary_ip expected to be basetypes.StringValue, was: %T`, primaryIpAttribute))
	}

	statusAttribute, ok := attributes["status"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`status is missing from object`)

		return nil, diags
	}

	statusVal, ok := statusAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`status expected to be basetypes.StringValue, was: %T`, statusAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return NodesValue{
		ApiResponding: apiRespondingVal,
		ClusterName:   clusterNameVal,
		ClusterUuid:   clusterUuidVal,
		Id:            idVal,
		IsMaster:      isMasterVal,
		Name:          nameVal,
		NodePoolName:  nodePoolNameVal,
		NodePoolUuid:  nodePoolUuidVal,
		PrimaryIp:     primaryIpVal,
		Status:        statusVal,
		state:         attr.ValueStateKnown,
	}, diags
}

func NewNodesValueNull() NodesValue {
	return NodesValue{
		state: attr.ValueStateNull,
	}
}

func NewNodesValueUnknown() NodesValue {
	return NodesValue{
		state: attr.ValueStateUnknown,
	}
}

func NewNodesValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (NodesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing NodesValue Attribute Value",
				"While creating a NodesValue value, a missing attribute value was detected. "+
					"A NodesValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("NodesValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid NodesValue Attribute Type",
				"While creating a NodesValue value, an invalid attribute value was detected. "+
					"A NodesValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("NodesValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("NodesValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra NodesValue Attribute Value",
				"While creating a NodesValue value, an extra attribute value was detected. "+
					"A NodesValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra NodesValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewNodesValueUnknown(), diags
	}

	apiRespondingAttribute, ok := attributes["api_responding"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`api_responding is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	apiRespondingVal, ok := apiRespondingAttribute.(basetypes.BoolValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`api_responding expected to be basetypes.BoolValue, was: %T`, apiRespondingAttribute))
	}

	clusterNameAttribute, ok := attributes["cluster_name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`cluster_name is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	clusterNameVal, ok := clusterNameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`cluster_name expected to be basetypes.StringValue, was: %T`, clusterNameAttribute))
	}

	clusterUuidAttribute, ok := attributes["cluster_uuid"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`cluster_uuid is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	clusterUuidVal, ok := clusterUuidAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`cluster_uuid expected to be basetypes.StringValue, was: %T`, clusterUuidAttribute))
	}

	idAttribute, ok := attributes["id"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`id is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	idVal, ok := idAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`id expected to be basetypes.StringValue, was: %T`, idAttribute))
	}

	isMasterAttribute, ok := attributes["is_master"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`is_master is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	isMasterVal, ok := isMasterAttribute.(basetypes.BoolValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`is_master expected to be basetypes.BoolValue, was: %T`, isMasterAttribute))
	}

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	nodePoolNameAttribute, ok := attributes["node_pool_name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`node_pool_name is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	nodePoolNameVal, ok := nodePoolNameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`node_pool_name expected to be basetypes.StringValue, was: %T`, nodePoolNameAttribute))
	}

	nodePoolUuidAttribute, ok := attributes["node_pool_uuid"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`node_pool_uuid is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	nodePoolUuidVal, ok := nodePoolUuidAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`node_pool_uuid expected to be basetypes.StringValue, was: %T`, nodePoolUuidAttribute))
	}

	primaryIpAttribute, ok := attributes["primary_ip"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`primary_ip is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	primaryIpVal, ok := primaryIpAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`primary_ip expected to be basetypes.StringValue, was: %T`, primaryIpAttribute))
	}

	statusAttribute, ok := attributes["status"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`status is missing from object`)

		return NewNodesValueUnknown(), diags
	}

	statusVal, ok := statusAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`status expected to be basetypes.StringValue, was: %T`, statusAttribute))
	}

	if diags.HasError() {
		return NewNodesValueUnknown(), diags
	}

	return NodesValue{
		ApiResponding: apiRespondingVal,
		ClusterName:   clusterNameVal,
		ClusterUuid:   clusterUuidVal,
		Id:            idVal,
		IsMaster:      isMasterVal,
		Name:          nameVal,
		NodePoolName:  nodePoolNameVal,
		NodePoolUuid:  nodePoolUuidVal,
		PrimaryIp:     primaryIpVal,
		Status:        statusVal,
		state:         attr.ValueStateKnown,
	}, diags
}

func NewNodesValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) NodesValue {
	object, diags := NewNodesValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewNodesValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t NodesType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewNodesValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewNodesValueUnknown(), nil
	}

	if in.IsNull() {
		return NewNodesValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewNodesValueMust(NodesValue{}.AttributeTypes(ctx), attributes), nil
}

func (t NodesType) ValueType(ctx context.Context) attr.Value {
	return NodesValue{}
}

var _ basetypes.ObjectValuable = NodesValue{}

type NodesValue struct {
	ApiResponding basetypes.BoolValue   `tfsdk:"api_responding"`
	ClusterName   basetypes.StringValue `tfsdk:"cluster_name"`
	ClusterUuid   basetypes.StringValue `tfsdk:"cluster_uuid"`
	Id            basetypes.StringValue `tfsdk:"id"`
	IsMaster      basetypes.BoolValue   `tfsdk:"is_master"`
	Name          basetypes.StringValue `tfsdk:"name"`
	NodePoolName  basetypes.StringValue `tfsdk:"node_pool_name"`
	NodePoolUuid  basetypes.StringValue `tfsdk:"node_pool_uuid"`
	PrimaryIp     basetypes.StringValue `tfsdk:"primary_ip"`
	Status        basetypes.StringValue `tfsdk:"status"`
	state         attr.ValueState
}

func (v NodesValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 10)

	var val tftypes.Value
	var err error

	attrTypes["api_responding"] = basetypes.BoolType{}.TerraformType(ctx)
	attrTypes["cluster_name"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["cluster_uuid"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["id"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["is_master"] = basetypes.BoolType{}.TerraformType(ctx)
	attrTypes["name"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["node_pool_name"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["node_pool_uuid"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["primary_ip"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["status"] = basetypes.StringType{}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 10)

		val, err = v.ApiResponding.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["api_responding"] = val

		val, err = v.ClusterName.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["cluster_name"] = val

		val, err = v.ClusterUuid.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["cluster_uuid"] = val

		val, err = v.Id.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["id"] = val

		val, err = v.IsMaster.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["is_master"] = val

		val, err = v.Name.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["name"] = val

		val, err = v.NodePoolName.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["node_pool_name"] = val

		val, err = v.NodePoolUuid.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["node_pool_uuid"] = val

		val, err = v.PrimaryIp.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["primary_ip"] = val

		val, err = v.Status.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["status"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v NodesValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v NodesValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v NodesValue) String() string {
	return "NodesValue"
}

func (v NodesValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	objVal, diags := types.ObjectValue(
		map[string]attr.Type{
			"api_responding": basetypes.BoolType{},
			"cluster_name":   basetypes.StringType{},
			"cluster_uuid":   basetypes.StringType{},
			"id":             basetypes.StringType{},
			"is_master":      basetypes.BoolType{},
			"name":           basetypes.StringType{},
			"node_pool_name": basetypes.StringType{},
			"node_pool_uuid": basetypes.StringType{},
			"primary_ip":     basetypes.StringType{},
			"status":         basetypes.StringType{},
		},
		map[string]attr.Value{
			"api_responding": v.ApiResponding,
			"cluster_name":   v.ClusterName,
			"cluster_uuid":   v.ClusterUuid,
			"id":             v.Id,
			"is_master":      v.IsMaster,
			"name":           v.Name,
			"node_pool_name": v.NodePoolName,
			"node_pool_uuid": v.NodePoolUuid,
			"primary_ip":     v.PrimaryIp,
			"status":         v.Status,
		})

	return objVal, diags
}

func (v NodesValue) Equal(o attr.Value) bool {
	other, ok := o.(NodesValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.ApiResponding.Equal(other.ApiResponding) {
		return false
	}

	if !v.ClusterName.Equal(other.ClusterName) {
		return false
	}

	if !v.ClusterUuid.Equal(other.ClusterUuid) {
		return false
	}

	if !v.Id.Equal(other.Id) {
		return false
	}

	if !v.IsMaster.Equal(other.IsMaster) {
		return false
	}

	if !v.Name.Equal(other.Name) {
		return false
	}

	if !v.NodePoolName.Equal(other.NodePoolName) {
		return false
	}

	if !v.NodePoolUuid.Equal(other.NodePoolUuid) {
		return false
	}

	if !v.PrimaryIp.Equal(other.PrimaryIp) {
		return false
	}

	if !v.Status.Equal(other.Status) {
		return false
	}

	return true
}

func (v NodesValue) Type(ctx context.Context) attr.Type {
	return NodesType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v NodesValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"api_responding": basetypes.BoolType{},
		"cluster_name":   basetypes.StringType{},
		"cluster_uuid":   basetypes.StringType{},
		"id":             basetypes.StringType{},
		"is_master":      basetypes.BoolType{},
		"name":           basetypes.StringType{},
		"node_pool_name": basetypes.StringType{},
		"node_pool_uuid": basetypes.StringType{},
		"primary_ip":     basetypes.StringType{},
		"status":         basetypes.StringType{},
	}
}
