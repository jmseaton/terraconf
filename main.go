package terraconf

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform/terraform"
	"github.com/hashicorp/terraform/flatmap"
	"github.com/hashicorp/hcl/hcl/printer"
)

const (
	tfStateKeyDelimiter = "."
)

type ResourceDefaults map[string]interface{}
type ResourceExcludes map[string]struct{}

func sanitizeResourceID(id string) string {
	return strings.Replace(id, tfStateKeyDelimiter, "_", -1)
}

func uniqueAttributeNames(attrMap map[string]string) map[string]bool {
	names := map[string]bool{}

	for k := range attrMap {
		name := strings.SplitN(k, tfStateKeyDelimiter, 2)[0]
		names[name] = false
	}

	return names
}

func IsPrimitive(rawValue interface{}) bool {
	switch rawValue.(type) {
	case string:
		return true
	case bool:
		return true
	case int:
		return true
	case int32:
		return true
	case int64:
		return true
	}

	return false
}

func PrimitiveValueToString(rawValue interface{}) string {
	switch v := rawValue.(type) {
	case string:
		// TODO: is it valid to always quote hcl strings?
		return strconv.Quote(v)
	case bool:
		return fmt.Sprintf("\"%t\"", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	}

	// TODO: handle unknown type
	return "unknown"
}

func PrimitiveAttributeToString(k string, rawValue interface{}) string {
	// TODO: how to handle empty string values? need more expressive way to exclude attributes?
	v := PrimitiveValueToString(rawValue)
	if k == "date" && v == "\"\"" {
		return ""
	}

	return fmt.Sprintf("%s = %s\n", k, v)
}

func PrimitiveAttributeListToString(attrName string, list []interface{}) string {
	s := fmt.Sprintf("%s = [\n", attrName)

	for _, v := range list {
		s += fmt.Sprintf("%s,", PrimitiveValueToString(v))
	}

	s += "]\n"

	return s
}

func MapAttributeToString(attrName string, m map[string]interface{}) string {
	s := fmt.Sprintf("%s {\n", attrName)

	for k, v := range m {
		if IsPrimitive(v) {
			s += PrimitiveAttributeToString(k, v)
		} else {
			s += AttributeToString(k, v)
		}
	}

	s += "}\n"

	return s
}

func AttributeToString(attrName string, attrRawVal interface{}) string {
	s := ""

	switch v := attrRawVal.(type) {
	case []interface{}:
		// TODO: option to include empty list/set, may cause issues when state has them

		if len(v) > 0 && IsPrimitive(v[0]) {
			s += PrimitiveAttributeListToString(attrName, v)
		} else {
			for _, item := range v {
				s += MapAttributeToString(attrName, item.(map[string]interface{}))
			}
		}
	case map[string]interface{}:
		// TODO: option to skip empty maps, may cause issues when state has them

		if len(v) > 0 {
			s += MapAttributeToString(attrName, v)
		}
	default:
		// Assuming primitive type string, bool, int, etc ...
		s += PrimitiveAttributeToString(attrName, v)
	}

	return s
}

// Given a ResourceState, overwrite the specified list attribute with the specified values.
func OverwriteList(state *terraform.ResourceState, attrName string, values interface{}) {
	newAttrs := flatmap.Flatten(map[string]interface{}{
		attrName: values,
	})

	attrs := flatmap.Map(state.Primary.Attributes)
	attrs.Delete(attrName)
	attrs.Merge(newAttrs)

	state.Primary.Attributes = attrs
}


func ResourceAsString(state *terraform.ResourceState) string {
	attrs := state.Primary.Attributes
	s := fmt.Sprintf("resource \"%s\" \"%s\" {\n", state.Type, state.Primary.ID)

	// We sort attribute names to make change diffs more consistent and easier to read.

	attrNames := uniqueAttributeNames(attrs)

	sortedAttrNames := []string{}
	for k := range attrNames {
		sortedAttrNames = append(sortedAttrNames, k)
	}
	sort.Strings(sortedAttrNames)

	for _, attrName := range sortedAttrNames {
		attrRawVal := flatmap.Expand(attrs, attrName)
		s += AttributeToString(attrName, attrRawVal)
	}

	if len(state.Dependencies) > 0 {
		s += "depends_on = [\n"
		for _, v := range state.Dependencies {
			s += PrimitiveValueToString(v)
		}
		s += "]\n"
	}

	s += "}\n"

	b, err := printer.Format([]byte(s))
	if err != nil {
		return ""
	}

	return string(b)
}

// features:
//     - sorted output to allow easy diff after running multiple times
//     - exclude map to exclude computed values
//     - auto excludes id
//     - default values allows config to generate correctly when the state doesn't have a value that will trigger change because default
//     - allow resource linking through interpolation, to let terraform generate correct dependency graph
// note:
//     - depends_on attributes not added since the state file lists calculated dependencies not just user set dependencies, maybe add option to generate
func ResourceStateToConfigString(state *terraform.ResourceState, defaults ResourceDefaults, excludes ResourceExcludes) string {
	attrs := state.Primary.Attributes

	// Note: The ID field for an individual resource state may not be safe and may contain periods.
	// At this point we do not have the safe ID anymore and must sanitize it. The only place the
	// safe ID exists is in the full state file as the keys of modules[].resources.
	s := fmt.Sprintf("resource \"%s\" \"%s\" {\n", state.Type, sanitizeResourceID(state.Primary.ID))

	// The id attribute should always be excluded.
	excludes["id"] = struct{}{}

	attrNames := uniqueAttributeNames(attrs)

	// Add the default if the attribute doesn't exist in the resource state.
	for attrName := range defaults {
		if _, ok := attrNames[attrName]; !ok {
			attrNames[attrName] = true
		}
	}

	// We sort attribute names to make change diffs more consistent and easier to read.
	sortedAttrNames := []string{}
	for k := range attrNames {
		sortedAttrNames = append(sortedAttrNames, k)
	}
	sort.Strings(sortedAttrNames)

	for _, attrName := range sortedAttrNames {
		if _, ok := excludes[attrName]; ok {
			continue
		}

		attrRawVal := flatmap.Expand(attrs, attrName)

		useDefault, _ := attrNames[attrName]
		defaultValue, defaultExists := defaults[attrName]

		if useDefault && defaultExists {
			s += AttributeToString(attrName, defaultValue)
			continue
		}

		s += AttributeToString(attrName, attrRawVal)
	}

	if len(state.Dependencies) > 0 {
		s += "depends_on = [\n"
		for _, v := range state.Dependencies {
			s += PrimitiveValueToString(v)
		}
		s += "]\n"
	}

	s += "}\n"

	b, err := printer.Format([]byte(s))
	if err != nil {
		return ""
	}

	return string(b)
}
