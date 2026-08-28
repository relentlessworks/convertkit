package converter

import (
	"strings"
	"testing"
)

func TestConvertJSONToYAML(t *testing.T) {
	input := `{"name":"test","value":42,"active":true}`
	output, err := Convert([]byte(input), FormatJSON, FormatYAML)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, "name: test") {
		t.Errorf("Expected 'name: test' in output, got: %s", str)
	}
	if !strings.Contains(str, "value: 42") {
		t.Errorf("Expected 'value: 42' in output, got: %s", str)
	}
	if !strings.Contains(str, "active: true") {
		t.Errorf("Expected 'active: true' in output, got: %s", str)
	}
}

func TestConvertYAMLToJSON(t *testing.T) {
	input := "name: test\nvalue: 42\nactive: true\n"
	output, err := Convert([]byte(input), FormatYAML, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"name": "test"`) {
		t.Errorf("Expected '\"name\": \"test\"' in output, got: %s", str)
	}
	if !strings.Contains(str, `"value": 42`) {
		t.Errorf("Expected '\"value\": 42' in output, got: %s", str)
	}
}

func TestConvertJSONToTOML(t *testing.T) {
	input := `{"title":"test","count":42,"active":true}`
	output, err := Convert([]byte(input), FormatJSON, FormatTOML)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `title = "test"`) {
		t.Errorf("Expected 'title = \"test\"' in output, got: %s", str)
	}
	if !strings.Contains(str, "count = 42") {
		t.Errorf("Expected 'count = 42' in output, got: %s", str)
	}
}

func TestConvertTOMLToJSON(t *testing.T) {
	input := `title = "test"
count = 42
active = true`
	output, err := Convert([]byte(input), FormatTOML, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"title": "test"`) {
		t.Errorf("Expected '\"title\": \"test\"' in output, got: %s", str)
	}
	if !strings.Contains(str, `"count": 42`) {
		t.Errorf("Expected '\"count\": 42' in output, got: %s", str)
	}
}

func TestConvertJSONToCSV(t *testing.T) {
	input := `[{"name":"alice","age":"30"},{"name":"bob","age":"25"}]`
	output, err := Convert([]byte(input), FormatJSON, FormatCSV)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	// Keys are sorted alphabetically: age,name
	if !strings.Contains(str, "age,name") {
		t.Errorf("Expected CSV header 'age,name', got: %s", str)
	}
	if !strings.Contains(str, "30,alice") {
		t.Errorf("Expected '30,alice' row, got: %s", str)
	}
}

func TestConvertCSVToJSON(t *testing.T) {
	input := "name,age\nalice,30\nbob,25\n"
	output, err := Convert([]byte(input), FormatCSV, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"name": "alice"`) {
		t.Errorf("Expected name=alice in output, got: %s", str)
	}
}

func TestConvertJSONToProperties(t *testing.T) {
	input := `{"app.name":"test","app.port":"8080","app.debug":"true"}`
	output, err := Convert([]byte(input), FormatJSON, FormatProperties)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, "app.name = test") {
		t.Errorf("Expected 'app.name = test', got: %s", str)
	}
}

func TestConvertPropertiesToJSON(t *testing.T) {
	input := "app.name = test\napp.port = 8080\n"
	output, err := Convert([]byte(input), FormatProperties, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"app.name": "test"`) {
		t.Errorf("Expected app.name=test, got: %s", str)
	}
}

func TestConvertJSONToJSONL(t *testing.T) {
	input := `[{"id":1,"name":"a"},{"id":2,"name":"b"}]`
	output, err := Convert([]byte(input), FormatJSON, FormatJSONL)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), output)
	}
}

func TestConvertJSONLToJSON(t *testing.T) {
	input := `{"id":1,"name":"a"}
{"id":2,"name":"b"}`
	output, err := Convert([]byte(input), FormatJSONL, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"id": 1`) {
		t.Errorf("Expected id=1, got: %s", str)
	}
}

func TestConvertJSONToEnv(t *testing.T) {
	input := `{"database":"test","port":"5432"}`
	output, err := Convert([]byte(input), FormatJSON, FormatEnv)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, "DATABASE=test") {
		t.Errorf("Expected DATABASE=test, got: %s", str)
	}
}

func TestConvertEnvToJSON(t *testing.T) {
	input := "DATABASE=test\nPORT=5432\n"
	output, err := Convert([]byte(input), FormatEnv, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"DATABASE": "test"`) {
		t.Errorf("Expected DATABASE=test, got: %s", str)
	}
}

func TestConvertJSONToXML(t *testing.T) {
	input := `{"name":"test","value":"42"}`
	output, err := Convert([]byte(input), FormatJSON, FormatXML)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, "<root>") {
		t.Errorf("Expected <root>, got: %s", str)
	}
	if !strings.Contains(str, "<name>test</name>") {
		t.Errorf("Expected <name>test</name>, got: %s", str)
	}
}

func TestConvertXMLToJSON(t *testing.T) {
	input := `<?xml version="1.0"?>
<root>
  <name>test</name>
  <value>42</value>
</root>`
	output, err := Convert([]byte(input), FormatXML, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"name": "test"`) {
		t.Errorf("Expected name=test, got: %s", str)
	}
}

func TestConvertInvalidFormat(t *testing.T) {
	_, err := Convert([]byte("test"), Format("ini"), FormatJSON)
	if err == nil {
		t.Error("Expected error for invalid format")
	}
}

func TestConvertRoundtripJSONYAML(t *testing.T) {
	input := `{"name":"test","value":42,"nested":{"key":"val"},"list":[1,2,3]}`
	// JSON → YAML
	yamlOut, err := Convert([]byte(input), FormatJSON, FormatYAML)
	if err != nil {
		t.Fatalf("JSON→YAML failed: %v", err)
	}
	// YAML → JSON
	jsonOut, err := Convert(yamlOut, FormatYAML, FormatJSON)
	if err != nil {
		t.Fatalf("YAML→JSON failed: %v", err)
	}
	str := string(jsonOut)
	if !strings.Contains(str, `"name": "test"`) {
		t.Errorf("Roundtrip failed, got: %s", str)
	}
	if !strings.Contains(str, `"value": 42`) {
		t.Errorf("Roundtrip failed, got: %s", str)
	}
}

func TestConvertYAMLWithNestedAndLists(t *testing.T) {
	input := `server:
  host: localhost
  port: 8080
features:
  - auth
  - logging
  - cache
`
	output, err := Convert([]byte(input), FormatYAML, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"host": "localhost"`) {
		t.Errorf("Expected host=localhost, got: %s", str)
	}
	if !strings.Contains(str, `"auth"`) {
		t.Errorf("Expected auth in list, got: %s", str)
	}
}

func TestConvertTOMLWithSections(t *testing.T) {
	input := `title = "TOML Example"

[server]
host = "localhost"
port = 8080

[[items]]
name = "first"

[[items]]
name = "second"
`
	output, err := Convert([]byte(input), FormatTOML, FormatJSON)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	str := string(output)
	if !strings.Contains(str, `"title": "TOML Example"`) {
		t.Errorf("Expected title, got: %s", str)
	}
	if !strings.Contains(str, `"host": "localhost"`) {
		t.Errorf("Expected host, got: %s", str)
	}
	if !strings.Contains(str, `"first"`) {
		t.Errorf("Expected first item, got: %s", str)
	}
}

func TestIsValidFormat(t *testing.T) {
	if !IsValidFormat("json") {
		t.Error("json should be valid")
	}
	if !IsValidFormat("yaml") {
		t.Error("yaml should be valid")
	}
	if IsValidFormat("ini") {
		t.Error("ini should not be valid")
	}
}

func TestSupportedFormats(t *testing.T) {
	formats := SupportedFormats()
	if len(formats) != 8 {
		t.Errorf("Expected 8 formats, got %d", len(formats))
	}
}
