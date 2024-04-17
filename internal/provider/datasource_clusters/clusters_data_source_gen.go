// Code generated by terraform-plugin-framework-generator DO NOT EDIT.

package datasource_clusters

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func ClustersDataSourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"cluster_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				Description:         "A list of cluster IDs for clusters that match all the specified filters.",
				MarkdownDescription: "A list of cluster IDs for clusters that match all the specified filters.",
			},
			"filters": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							Description:         "Name of the attribute on which this filter should be applied. The tags attribute is a special case where the name has to be specified as 'tags:<tag_key>'",
							MarkdownDescription: "Name of the attribute on which this filter should be applied. The tags attribute is a special case where the name has to be specified as 'tags:<tag_key>'",
						},
						"regexes": schema.SetAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							Description:         "Set of regexes to match to the attribute value, if any of the 'regex' matches then the filter is considered to be matched",
							MarkdownDescription: "Set of regexes to match to the attribute value, if any of the 'regex' matches then the filter is considered to be matched",
						},
						"values": schema.SetAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							Description:         "Set of values for the attribute, if any of the 'value' matches then the filter is considered to be matched",
							MarkdownDescription: "Set of values for the attribute, if any of the 'value' matches then the filter is considered to be matched",
						},
					},
					CustomType: FiltersType{
						ObjectType: types.ObjectType{
							AttrTypes: FiltersValue{}.AttributeTypes(ctx),
						},
					},
				},
				Optional:            true,
				Description:         "List of filters, each filter is applied to the list of clusters one by one. Hence the results match all the specified filters. All the filters are ANDed.",
				MarkdownDescription: "List of filters, each filter is applied to the list of clusters one by one. Hence the results match all the specified filters. All the filters are ANDed.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Placeholder for an ID",
				MarkdownDescription: "Placeholder for an ID",
			},
		},
	}
}

type ClustersModel struct {
	ClusterIds types.List   `tfsdk:"cluster_ids"`
	Filters    types.List   `tfsdk:"filters"`
	Id         types.String `tfsdk:"id"`
}

var _ basetypes.ObjectTypable = FiltersType{}

type FiltersType struct {
	basetypes.ObjectType
}

func (t FiltersType) Equal(o attr.Type) bool {
	other, ok := o.(FiltersType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t FiltersType) String() string {
	return "FiltersType"
}

func (t FiltersType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
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

	regexesAttribute, ok := attributes["regexes"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`regexes is missing from object`)

		return nil, diags
	}

	regexesVal, ok := regexesAttribute.(basetypes.SetValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`regexes expected to be basetypes.SetValue, was: %T`, regexesAttribute))
	}

	valuesAttribute, ok := attributes["values"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`values is missing from object`)

		return nil, diags
	}

	valuesVal, ok := valuesAttribute.(basetypes.SetValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`values expected to be basetypes.SetValue, was: %T`, valuesAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return FiltersValue{
		Name:    nameVal,
		Regexes: regexesVal,
		Values:  valuesVal,
		state:   attr.ValueStateKnown,
	}, diags
}

func NewFiltersValueNull() FiltersValue {
	return FiltersValue{
		state: attr.ValueStateNull,
	}
}

func NewFiltersValueUnknown() FiltersValue {
	return FiltersValue{
		state: attr.ValueStateUnknown,
	}
}

func NewFiltersValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (FiltersValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing FiltersValue Attribute Value",
				"While creating a FiltersValue value, a missing attribute value was detected. "+
					"A FiltersValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid FiltersValue Attribute Type",
				"While creating a FiltersValue value, an invalid attribute value was detected. "+
					"A FiltersValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra FiltersValue Attribute Value",
				"While creating a FiltersValue value, an extra attribute value was detected. "+
					"A FiltersValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra FiltersValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewFiltersValueUnknown(), diags
	}

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return NewFiltersValueUnknown(), diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	regexesAttribute, ok := attributes["regexes"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`regexes is missing from object`)

		return NewFiltersValueUnknown(), diags
	}

	regexesVal, ok := regexesAttribute.(basetypes.SetValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`regexes expected to be basetypes.SetValue, was: %T`, regexesAttribute))
	}

	valuesAttribute, ok := attributes["values"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`values is missing from object`)

		return NewFiltersValueUnknown(), diags
	}

	valuesVal, ok := valuesAttribute.(basetypes.SetValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`values expected to be basetypes.SetValue, was: %T`, valuesAttribute))
	}

	if diags.HasError() {
		return NewFiltersValueUnknown(), diags
	}

	return FiltersValue{
		Name:    nameVal,
		Regexes: regexesVal,
		Values:  valuesVal,
		state:   attr.ValueStateKnown,
	}, diags
}

func NewFiltersValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) FiltersValue {
	object, diags := NewFiltersValue(attributeTypes, attributes)

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

		panic("NewFiltersValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t FiltersType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewFiltersValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewFiltersValueUnknown(), nil
	}

	if in.IsNull() {
		return NewFiltersValueNull(), nil
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

	return NewFiltersValueMust(FiltersValue{}.AttributeTypes(ctx), attributes), nil
}

func (t FiltersType) ValueType(ctx context.Context) attr.Value {
	return FiltersValue{}
}

var _ basetypes.ObjectValuable = FiltersValue{}

type FiltersValue struct {
	Name    basetypes.StringValue `tfsdk:"name"`
	Regexes basetypes.SetValue    `tfsdk:"regexes"`
	Values  basetypes.SetValue    `tfsdk:"values"`
	state   attr.ValueState
}

func (v FiltersValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 3)

	var val tftypes.Value
	var err error

	attrTypes["name"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["regexes"] = basetypes.SetType{
		ElemType: types.StringType,
	}.TerraformType(ctx)
	attrTypes["values"] = basetypes.SetType{
		ElemType: types.StringType,
	}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 3)

		val, err = v.Name.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["name"] = val

		val, err = v.Regexes.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["regexes"] = val

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

func (v FiltersValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v FiltersValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v FiltersValue) String() string {
	return "FiltersValue"
}

func (v FiltersValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	regexesVal, d := types.SetValue(types.StringType, v.Regexes.Elements())

	diags.Append(d...)

	if d.HasError() {
		return types.ObjectUnknown(map[string]attr.Type{
			"name": basetypes.StringType{},
			"regexes": basetypes.SetType{
				ElemType: types.StringType,
			},
			"values": basetypes.SetType{
				ElemType: types.StringType,
			},
		}), diags
	}

	valuesVal, d := types.SetValue(types.StringType, v.Values.Elements())

	diags.Append(d...)

	if d.HasError() {
		return types.ObjectUnknown(map[string]attr.Type{
			"name": basetypes.StringType{},
			"regexes": basetypes.SetType{
				ElemType: types.StringType,
			},
			"values": basetypes.SetType{
				ElemType: types.StringType,
			},
		}), diags
	}

	objVal, diags := types.ObjectValue(
		map[string]attr.Type{
			"name": basetypes.StringType{},
			"regexes": basetypes.SetType{
				ElemType: types.StringType,
			},
			"values": basetypes.SetType{
				ElemType: types.StringType,
			},
		},
		map[string]attr.Value{
			"name":    v.Name,
			"regexes": regexesVal,
			"values":  valuesVal,
		})

	return objVal, diags
}

func (v FiltersValue) Equal(o attr.Value) bool {
	other, ok := o.(FiltersValue)

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

	if !v.Regexes.Equal(other.Regexes) {
		return false
	}

	if !v.Values.Equal(other.Values) {
		return false
	}

	return true
}

func (v FiltersValue) Type(ctx context.Context) attr.Type {
	return FiltersType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v FiltersValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"name": basetypes.StringType{},
		"regexes": basetypes.SetType{
			ElemType: types.StringType,
		},
		"values": basetypes.SetType{
			ElemType: types.StringType,
		},
	}
}
