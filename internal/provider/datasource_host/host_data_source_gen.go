// Code generated by terraform-plugin-framework-generator DO NOT EDIT.

package datasource_host

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

func HostDataSourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"arch": schema.StringAttribute{
				Computed:            true,
				Description:         "Architecture of the host",
				MarkdownDescription: "Architecture of the host",
			},
			"hostname": schema.StringAttribute{
				Computed:            true,
				Description:         "Name of the host",
				MarkdownDescription: "Name of the host",
			},
			"id": schema.StringAttribute{
				Required:            true,
				Description:         "UUID of the host",
				MarkdownDescription: "UUID of the host",
			},
			"interfaces": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip": schema.StringAttribute{
							Computed:            true,
							Description:         "IP address of the interface",
							MarkdownDescription: "IP address of the interface",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							Description:         "Name of the interface",
							MarkdownDescription: "Name of the interface",
						},
					},
					CustomType: InterfacesType{
						ObjectType: types.ObjectType{
							AttrTypes: InterfacesValue{}.AttributeTypes(ctx),
						},
					},
				},
				Computed: true,
			},
			"last_response_time": schema.StringAttribute{
				Computed:            true,
				Description:         "Time of the last response from the host",
				MarkdownDescription: "Time of the last response from the host",
			},
			"message": schema.StringAttribute{
				Computed: true,
			},
			"os_family": schema.StringAttribute{
				Computed:            true,
				Description:         "Operating system family of the host",
				MarkdownDescription: "Operating system family of the host",
			},
			"os_info": schema.StringAttribute{
				Computed:            true,
				Description:         "Operating system information",
				MarkdownDescription: "Operating system information",
			},
			"responding": schema.BoolAttribute{
				Computed:            true,
				Description:         "Indicates if the host is responding",
				MarkdownDescription: "Indicates if the host is responding",
			},
			"role_status": schema.StringAttribute{
				Computed:            true,
				Description:         "Status of the role",
				MarkdownDescription: "Status of the role",
			},
			"roles": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				Description:         "Roles of the host",
				MarkdownDescription: "Roles of the host",
			},
		},
	}
}

type HostModel struct {
	Arch             types.String `tfsdk:"arch"`
	Hostname         types.String `tfsdk:"hostname"`
	Id               types.String `tfsdk:"id"`
	Interfaces       types.List   `tfsdk:"interfaces"`
	LastResponseTime types.String `tfsdk:"last_response_time"`
	Message          types.String `tfsdk:"message"`
	OsFamily         types.String `tfsdk:"os_family"`
	OsInfo           types.String `tfsdk:"os_info"`
	Responding       types.Bool   `tfsdk:"responding"`
	RoleStatus       types.String `tfsdk:"role_status"`
	Roles            types.List   `tfsdk:"roles"`
}

var _ basetypes.ObjectTypable = InterfacesType{}

type InterfacesType struct {
	basetypes.ObjectType
}

func (t InterfacesType) Equal(o attr.Type) bool {
	other, ok := o.(InterfacesType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t InterfacesType) String() string {
	return "InterfacesType"
}

func (t InterfacesType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	ipAttribute, ok := attributes["ip"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`ip is missing from object`)

		return nil, diags
	}

	ipVal, ok := ipAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`ip expected to be basetypes.StringValue, was: %T`, ipAttribute))
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

	if diags.HasError() {
		return nil, diags
	}

	return InterfacesValue{
		Ip:    ipVal,
		Name:  nameVal,
		state: attr.ValueStateKnown,
	}, diags
}

func NewInterfacesValueNull() InterfacesValue {
	return InterfacesValue{
		state: attr.ValueStateNull,
	}
}

func NewInterfacesValueUnknown() InterfacesValue {
	return InterfacesValue{
		state: attr.ValueStateUnknown,
	}
}

func NewInterfacesValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (InterfacesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing InterfacesValue Attribute Value",
				"While creating a InterfacesValue value, a missing attribute value was detected. "+
					"A InterfacesValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("InterfacesValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid InterfacesValue Attribute Type",
				"While creating a InterfacesValue value, an invalid attribute value was detected. "+
					"A InterfacesValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("InterfacesValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("InterfacesValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra InterfacesValue Attribute Value",
				"While creating a InterfacesValue value, an extra attribute value was detected. "+
					"A InterfacesValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra InterfacesValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewInterfacesValueUnknown(), diags
	}

	ipAttribute, ok := attributes["ip"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`ip is missing from object`)

		return NewInterfacesValueUnknown(), diags
	}

	ipVal, ok := ipAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`ip expected to be basetypes.StringValue, was: %T`, ipAttribute))
	}

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return NewInterfacesValueUnknown(), diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	if diags.HasError() {
		return NewInterfacesValueUnknown(), diags
	}

	return InterfacesValue{
		Ip:    ipVal,
		Name:  nameVal,
		state: attr.ValueStateKnown,
	}, diags
}

func NewInterfacesValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) InterfacesValue {
	object, diags := NewInterfacesValue(attributeTypes, attributes)

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

		panic("NewInterfacesValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t InterfacesType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewInterfacesValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewInterfacesValueUnknown(), nil
	}

	if in.IsNull() {
		return NewInterfacesValueNull(), nil
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

	return NewInterfacesValueMust(InterfacesValue{}.AttributeTypes(ctx), attributes), nil
}

func (t InterfacesType) ValueType(ctx context.Context) attr.Value {
	return InterfacesValue{}
}

var _ basetypes.ObjectValuable = InterfacesValue{}

type InterfacesValue struct {
	Ip    basetypes.StringValue `tfsdk:"ip"`
	Name  basetypes.StringValue `tfsdk:"name"`
	state attr.ValueState
}

func (v InterfacesValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 2)

	var val tftypes.Value
	var err error

	attrTypes["ip"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["name"] = basetypes.StringType{}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 2)

		val, err = v.Ip.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["ip"] = val

		val, err = v.Name.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["name"] = val

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

func (v InterfacesValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v InterfacesValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v InterfacesValue) String() string {
	return "InterfacesValue"
}

func (v InterfacesValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	objVal, diags := types.ObjectValue(
		map[string]attr.Type{
			"ip":   basetypes.StringType{},
			"name": basetypes.StringType{},
		},
		map[string]attr.Value{
			"ip":   v.Ip,
			"name": v.Name,
		})

	return objVal, diags
}

func (v InterfacesValue) Equal(o attr.Value) bool {
	other, ok := o.(InterfacesValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.Ip.Equal(other.Ip) {
		return false
	}

	if !v.Name.Equal(other.Name) {
		return false
	}

	return true
}

func (v InterfacesValue) Type(ctx context.Context) attr.Type {
	return InterfacesType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v InterfacesValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"ip":   basetypes.StringType{},
		"name": basetypes.StringType{},
	}
}
