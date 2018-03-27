package terraconf

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform/terraform"
)

// Test sanitizeResourceID

func TestSanitizeResourceIDWithPeriods(t *testing.T) {
	result := sanitizeResourceID("my.test.resource.name")
	expected := "my_test_resource_name"

	if result != expected {
		t.Errorf("Expected '%s' but go '%s'", expected, result)
	}
}

// Test getUniqueAttributeNames

func TestGetUniqueAttributeNamesWithDupes(t *testing.T) {
	attrMap := map[string]string{
		"name":                      "test",
		"environment.#":             "1",
		"environment.0.variables.%": "1",
		"environment.0.variables.a": "value",
	}

	result := getUniqueAttributeNames(attrMap)

	keys := []string{
		"name",
		"environment",
	}

	for _, key := range keys {
		if _, ok := result[key]; !ok {
			t.Errorf("Expected %v to contain '%s'", attrMap, key)
		}
	}
}

func TestGetUniqueAttributeNamesWithoutDupes(t *testing.T) {
	attrMap := map[string]string{
		"name":   "test",
		"tags.%": "0",
	}

	result := getUniqueAttributeNames(attrMap)

	keys := []string{
		"name",
		"tags",
	}

	for _, key := range keys {
		if _, ok := result[key]; !ok {
			t.Errorf("Expected %v to contain '%s'", attrMap, key)
		}
	}
}

// Test isPrimitive

func TestIsPrimitiveString(t *testing.T) {
	rawValue := "mystring"
	result := isPrimitive(rawValue)

	if !result {
		t.Errorf("Expected %t for %T but got %t", true, rawValue, result)
	}
}

func TestIsPrimitiveBool(t *testing.T) {
	rawValues := []interface{}{
		true,
		false,
	}

	for _, rawValue := range rawValues {
		result := isPrimitive(rawValue)

		if !result {
			t.Errorf("Expected %t for %T but got %t", true, rawValue, result)
		}
	}
}

func TestIsPrimitiveInts(t *testing.T) {
	rawValues := []interface{}{
		int(1),
		int8(8),
		int16(16),
		int32(32),
		int64(64),
	}

	for _, rawValue := range rawValues {
		result := isPrimitive(rawValue)

		if !result {
			t.Errorf("Expected %t for %T but got %t", true, rawValue, result)
		}
	}
}

func TestIsPrimitiveFalse(t *testing.T) {
	rawValues := []interface{}{
		map[string]string{},
		[]string{},
		[]*string{},
		[]byte("mystring"),
		struct{}{},
	}

	for _, rawValue := range rawValues {
		result := isPrimitive(rawValue)

		if result {
			t.Errorf("Expected %t for %T but got %t", false, rawValue, result)
		}
	}
}

// Test getPrimitiveValueString

func TestGetPrimitiveValueStringWithString(t *testing.T) {
	values := []string{
		"mystring",
		"\"mykey\": \"myvalue\"",
	}

	for _, value := range values {
		result := getPrimitiveValueString(value)
		expected := strconv.Quote(value)

		if result != expected {
			t.Errorf("Expected '%s' for '%s' but got '%s'", expected, value, result)
		}
	}
}

func TestGetPrimitiveValueStringWithBool(t *testing.T) {
	values := []bool{
		true,
		false,
	}

	for _, value := range values {
		result := getPrimitiveValueString(value)
		expected := fmt.Sprintf("\"%t\"", value)

		if result != expected {
			t.Errorf("Expected '%s' for %t but got '%s'", expected, value, result)
		}
	}
}

func TestGetPrimitiveValueStringWithInts(t *testing.T) {
	rawValues := []interface{}{
		int(1),
		int8(8),
		int16(16),
		int32(32),
		int64(64),
	}

	for _, rawValue := range rawValues {
		result := getPrimitiveValueString(rawValue)
		expected := fmt.Sprintf("%d", rawValue)

		if result != expected {
			t.Errorf("Expected '%s' for %v but got '%s'", expected, rawValue, result)
		}
	}
}

func TestGetPrimitiveValueStringWithNonPrimitive(t *testing.T) {
	rawValues := []interface{}{
		map[string]string{},
		[]string{},
		[]*string{},
		[]byte("mystring"),
		struct{}{},
	}

	for _, rawValue := range rawValues {
		result := getPrimitiveValueString(rawValue)

		if result != "unknown" {
			t.Errorf("Expected 'unknown' for %v but got '%s'", rawValue, result)
		}
	}
}

// Test getDependsString

func TestGetDependsStringWithoutDeps(t *testing.T) {
	result := getDependsString([]string{})

	if result != "" {
		t.Errorf("Expected empty string but got '%s'", result)
	}
}

func TestGetDependsStringWithDeps(t *testing.T) {
	result := getDependsString([]string{
		"mydependency",
	})
	expected := "depends_on = [\n\"mydependency\"]\n"

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

// Test getPrimitiveAttributeString

func TestGetPrimitiveAttributeStringWithString(t *testing.T) {
	key := "mykey"
	values := []string{
		"myvalue",
		"\"mykey\": \"myvalue\"",
	}

	for _, value := range values {
		result := getPrimitiveAttributeString(key, value)
		expected := fmt.Sprintf("%s = %s\n", key, strconv.Quote(value))

		if result != expected {
			t.Errorf("Expected '%s' but got '%s'", expected, result)
		}
	}
}

func TestGetPrimitiveAttributeStringWithBool(t *testing.T) {
	key := "mykey"
	values := []bool{
		true,
		false,
	}

	for _, value := range values {
		result := getPrimitiveAttributeString(key, value)
		expected := fmt.Sprintf("%s = \"%t\"\n", key, value)

		if result != expected {
			t.Errorf("Expected '%s' but got '%s'", expected, result)
		}
	}
}

func TestGetPrimitiveAttributeStringWithInts(t *testing.T) {
	key := "mykey"
	values := []interface{}{
		int(1),
		int8(8),
		int16(16),
		int32(32),
		int64(64),
	}

	for _, value := range values {
		result := getPrimitiveAttributeString(key, value)
		expected := fmt.Sprintf("%s = %d\n", key, value)

		if result != expected {
			t.Errorf("Expected '%s' but got '%s'", expected, result)
		}
	}
}

func TestGetPrimitiveAttributeStringWithDateKey(t *testing.T) {
	result := getPrimitiveAttributeString("date", "")

	if result != "" {
		t.Errorf("Expected empty string but got '%s'", result)
	}
}

// Test getPrimitiveAttributeListString
// TODO: Add more complete tests

func TestGetPrimitiveAttributeListString(t *testing.T) {
	values := []interface{}{
		"sg-xxxxxxxx",
		"sg-xxxxxxxx",
	}

	result := getPrimitiveAttributeListString("security_groups", values)
	expected := "security_groups = [\n\"sg-xxxxxxxx\",\"sg-xxxxxxxx\",]\n"

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

// Test getMapAttributeString
// TODO: Add more complete tests
// TODO: Found bug, map string should be sorted to avoid diffs, for now checking for both orders

func TestGetMapAttributeString(t *testing.T) {
	m := map[string]interface{}{
		"a": "a",
		"b": "b",
	}

	result := getMapAttributeString("tags", m)
	expectedA := "tags {\na = \"a\"\nb = \"b\"\n}\n"
	expectedB := "tags {\nb = \"b\"\na = \"a\"\n}\n"

	if result != expectedA && result != expectedB {
		t.Errorf("Expected '%s' but got '%s'", expectedA, result)
	}
}

// Test formatConfig
// TODO: Determine how thorough these tests should be. It mostly tests printer.Format

func TestFormatConfig(t *testing.T) {
	result := formatConfig("resource \"type\" \"name\" {\n\n}")
	expected := "resource \"type\" \"name\" {}\n"

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

// Test getAttributeString
// TODO: Add more complete tests

func TestGetAttributeStringPrimitive(t *testing.T) {
	result := getAttributeString("name", "myname")
	expected := "name = \"myname\"\n"

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

// TODO: found bug or reason to fix unknown case of getPrimitiveValueString
func TestGetAttributeStringList(t *testing.T) {
	result := getAttributeString("names", []interface{}{"a", "b"})
	expected := "names = [\n\"a\",\"b\",]\n"

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

// TODO: found bug or reason to fix unknown case of getPrimitiveValueString
func TestGetAttributeStringMap(t *testing.T) {
	result := getAttributeString("names", map[string]interface{}{"a": "a", "b": "b"})
	expectedA := "names {\na = \"a\"\nb = \"b\"\n}\n"
	expectedB := "names {\nb = \"b\"\na = \"a\"\n}\n"

	if result != expectedA && result != expectedB {
		t.Errorf("Expected '%s' but got '%s'", expectedA, result)
	}
}

// Test GetResourceStateConfigString
// TODO: Add more complete tests

func TestGetResourceStateConfigString(t *testing.T) {
	state := &terraform.ResourceState{
		Type:         "resource_type",
		Dependencies: []string{"mydep"},
		Primary: &terraform.InstanceState{
			ID: "my.resource",
			Attributes: map[string]string{
				"name":         "myname",
				"myexcludekey": "myexcludeval",
			},
		},
	}

	result := GetResourceStateConfigString(state, ResourceDefaults{}, ResourceExcludes{"myexcludekey": struct{}{}})
	expected := "resource \"resource_type\" \"my_resource\" {\n  name = \"myname\"\n\n  depends_on = [\n    \"mydep\",\n  ]\n}\n"

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}
