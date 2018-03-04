package terraconf

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/hcl/printer"
	"github.com/hashicorp/terraform/flatmap"
	"github.com/hashicorp/terraform/terraform"
)

const (
	tfStateKeyDelimiter            = "."
	tfStateKeyDelimiterReplacement = "_"
)

type ResourceDefaults map[string]interface{}
type ResourceExcludes map[string]struct{}

// Replace attribute key separator character to create a name that is safe to use as a resource name.
func sanitizeResourceID(id string) string {
	return strings.Replace(id, tfStateKeyDelimiter, tfStateKeyDelimiterReplacement, -1)
}

// Given a state attribute map that contains complex key names like tags.%, generate a map of
// unique base attribute names like [tags: false].
func getUniqueAttributeNames(attrMap map[string]string) map[string]bool {
	names := map[string]bool{}

	for k := range attrMap {
		name := strings.SplitN(k, tfStateKeyDelimiter, 2)[0]
		names[name] = false
	}

	return names
}

func isPrimitive(rawValue interface{}) bool {
	switch rawValue.(type) {
	case string:
		return true
	case bool:
		return true
	case int, int8, int16, int32, int64:
		return true
	}

	return false
}

func getPrimitiveValueString(rawValue interface{}) (s string) {
	switch v := rawValue.(type) {
	case string:
		// TODO: is it valid to always quote hcl strings?
		return strconv.Quote(v)
	case bool:
		return fmt.Sprintf("\"%t\"", v)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	}

	// TODO: handle unknown type
	return "unknown"
}

func getDependsString(dependencies []string) (s string) {
	if len(dependencies) > 0 {
		s += "depends_on = [\n"
		for _, v := range dependencies {
			s += getPrimitiveValueString(v)
		}
		s += "]\n"
	}

	return
}

func getPrimitiveAttributeString(k string, rawValue interface{}) string {
	// TODO: how to handle empty string values? need more expressive way to exclude attributes?
	v := getPrimitiveValueString(rawValue)
	// TODO: remember use case for this and generalize
	if k == "date" && v == "\"\"" {
		return ""
	}

	return fmt.Sprintf("%s = %s\n", k, v)
}

func getPrimitiveAttributeListString(attrName string, list []interface{}) string {
	s := fmt.Sprintf("%s = [\n", attrName)

	for _, v := range list {
		s += fmt.Sprintf("%s,", getPrimitiveValueString(v))
	}

	s += "]\n"

	return s
}

func getMapAttributeString(attrName string, m map[string]interface{}) string {
	s := fmt.Sprintf("%s {\n", attrName)

	for k, v := range m {
		if isPrimitive(v) {
			s += getPrimitiveAttributeString(k, v)
		} else {
			s += getAttributeString(k, v)
		}
	}

	s += "}\n"

	return s
}

func getAttributeString(attrName string, attrRawVal interface{}) string {
	s := ""

	switch v := attrRawVal.(type) {
	case []interface{}:
		// TODO: option to include empty list/set, may cause issues when state has them

		if len(v) > 0 && isPrimitive(v[0]) {
			s += getPrimitiveAttributeListString(attrName, v)
		} else {
			for _, item := range v {
				s += getMapAttributeString(attrName, item.(map[string]interface{}))
			}
		}
	case map[string]interface{}:
		// TODO: option to skip empty maps, may cause issues when state has them

		if len(v) > 0 {
			s += getMapAttributeString(attrName, v)
		}
	default:
		// Assuming primitive type string, bool, int, etc ...
		s += getPrimitiveAttributeString(attrName, v)
	}

	return s
}

// Given a ResourceState, overwrite the specified list attribute with the specified values.
//func overwriteList(state *terraform.ResourceState, attrName string, values interface{}) {
//	newAttrs := flatmap.Flatten(map[string]interface{}{
//		attrName: values,
//	})
//
//	attrs := flatmap.Map(state.Primary.Attributes)
//	attrs.Delete(attrName)
//	attrs.Merge(newAttrs)
//
//	state.Primary.Attributes = attrs
//}

func formatConfig(s string) string {
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
func GetResourceStateConfigString(state *terraform.ResourceState, defaults ResourceDefaults, excludes ResourceExcludes) string {
	attrs := state.Primary.Attributes

	// Note: The ID field for an individual resource state may not be safe and may contain periods.
	// At this point we do not have the safe ID anymore and must sanitize it. The only place the
	// safe ID exists is in the full state file as the keys of modules[].resources.
	s := fmt.Sprintf("resource \"%s\" \"%s\" {\n", state.Type, sanitizeResourceID(state.Primary.ID))

	// The id attribute should always be excluded.
	excludes["id"] = struct{}{}

	attrNames := getUniqueAttributeNames(attrs)

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
			s += getAttributeString(attrName, defaultValue)
			continue
		}

		s += getAttributeString(attrName, attrRawVal)
	}

	s += getDependsString(state.Dependencies)

	s += "}\n"

	return formatConfig(s)
}
