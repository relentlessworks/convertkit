package converter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Format represents a supported data format.
type Format string

const (
	FormatJSON       Format = "json"
	FormatYAML       Format = "yaml"
	FormatTOML       Format = "toml"
	FormatCSV        Format = "csv"
	FormatXML        Format = "xml"
	FormatProperties Format = "properties"
	FormatJSONL      Format = "jsonl"
	FormatEnv        Format = "env"
)

// SupportedFormats returns all formats this converter can handle.
func SupportedFormats() []Format {
	return []Format{
		FormatJSON, FormatYAML, FormatTOML, FormatCSV,
		FormatXML, FormatProperties, FormatJSONL, FormatEnv,
	}
}

// FormatDescription returns a human-readable description of a format.
func FormatDescription(f Format) string {
	switch f {
	case FormatJSON:
		return "JavaScript Object Notation — the lingua franca of APIs"
	case FormatYAML:
		return "YAML Ain't Markup Language — human-friendly config format"
	case FormatTOML:
		return "Tom's Obvious Minimal Language — config format with types"
	case FormatCSV:
		return "Comma-Separated Values — tabular data, one row per line"
	case FormatXML:
		return "eXtensible Markup Language — hierarchical document format"
	case FormatProperties:
		return "Java Properties — key=value pairs, one per line"
	case FormatJSONL:
		return "JSON Lines — one JSON object per line (for log/stream data)"
	case FormatEnv:
		return "Environment variables — KEY=value shell-style pairs"
	default:
		return "unknown format"
	}
}

// IsValidFormat checks if a format string is supported.
func IsValidFormat(f string) bool {
	for _, sf := range SupportedFormats() {
		if string(sf) == f {
			return true
		}
	}
	return false
}

// Convert converts data from one format to another.
func Convert(data []byte, from, to Format) ([]byte, error) {
	if !IsValidFormat(string(from)) {
		return nil, fmt.Errorf("unsupported source format: %s | hint: use one of: %s", from, formatList())
	}
	if !IsValidFormat(string(to)) {
		return nil, fmt.Errorf("unsupported target format: %s | hint: use one of: %s", to, formatList())
	}

	v, err := Parse(data, from)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %v | hint: check that your input is valid %s", from, err, from)
	}

	out, err := Serialize(v, to)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize to %s: %v | hint: the data structure may not be representable in %s", to, err, to)
	}

	return out, nil
}

// Parse converts raw bytes in a given format to a Go interface{}.
func Parse(data []byte, f Format) (interface{}, error) {
	switch f {
	case FormatJSON:
		return parseJSON(data)
	case FormatYAML:
		return parseYAMLSimple(data)
	case FormatTOML:
		return parseTOMLSimple(data)
	case FormatCSV:
		return parseCSV(data)
	case FormatXML:
		return parseXMLSimple(data)
	case FormatProperties:
		return parseProperties(data)
	case FormatJSONL:
		return parseJSONL(data)
	case FormatEnv:
		return parseEnv(data)
	default:
		return nil, fmt.Errorf("unsupported format: %s", f)
	}
}

// Serialize converts a Go interface{} to bytes in a given format.
func Serialize(v interface{}, f Format) ([]byte, error) {
	switch f {
	case FormatJSON:
		return serializeJSON(v)
	case FormatYAML:
		return serializeYAMLSimple(v, 0)
	case FormatTOML:
		return serializeTOMLSimple(v)
	case FormatCSV:
		return serializeCSV(v)
	case FormatXML:
		return serializeXMLSimple(v)
	case FormatProperties:
		return serializeProperties(v)
	case FormatJSONL:
		return serializeJSONL(v)
	case FormatEnv:
		return serializeEnv(v)
	default:
		return nil, fmt.Errorf("unsupported format: %s", f)
	}
}

func formatList() string {
	formats := SupportedFormats()
	parts := make([]string, len(formats))
	for i, f := range formats {
		parts[i] = string(f)
	}
	return strings.Join(parts, ", ")
}

func sortStrings(s []string) {
	sort.Strings(s)
}

// --- JSON ---

func parseJSON(data []byte) (interface{}, error) {
	var v interface{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return normalizeNumbers(v), nil
}

func serializeJSON(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func normalizeNumbers(v interface{}) interface{} {
	switch val := v.(type) {
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i
		}
		f, _ := val.Float64()
		return f
	case map[string]interface{}:
		for k, vv := range val {
			val[k] = normalizeNumbers(vv)
		}
		return val
	case []interface{}:
		for i, vv := range val {
			val[i] = normalizeNumbers(vv)
		}
		return val
	default:
		return v
	}
}

// --- CSV ---

func parseCSV(data []byte) (interface{}, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []interface{}{}, nil
	}

	header := records[0]
	var result []interface{}
	for i := 1; i < len(records); i++ {
		row := make(map[string]interface{})
		for j, col := range records[i] {
			if j < len(header) {
				row[header[j]] = col
			}
		}
		result = append(result, row)
	}
	if result == nil {
		result = []interface{}{}
	}
	return result, nil
}

func serializeCSV(v interface{}) ([]byte, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("CSV requires an array of objects")
	}
	if len(arr) == 0 {
		return []byte{}, nil
	}

	keySet := make(map[string]bool)
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			for k := range m {
				keySet[k] = true
			}
		}
	}
	if len(keySet) == 0 {
		return nil, fmt.Errorf("CSV requires array of objects with keys")
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Write(keys)

	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("CSV requires array of objects, found %T", item)
		}
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = toString(m[k])
		}
		writer.Write(row)
	}
	writer.Flush()
	return buf.Bytes(), nil
}

// --- Properties ---

func parseProperties(data []byte) (interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		idx := strings.IndexAny(line, "=:")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		result[key] = val
	}
	return result, nil
}

func serializeProperties(v interface{}) ([]byte, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("properties format requires a flat key-value object")
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(" = ")
		buf.WriteString(toString(m[k]))
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// --- JSONL ---

func parseJSONL(data []byte) (interface{}, error) {
	var result []interface{}
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var v interface{}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("invalid JSON on line: %v", err)
		}
		result = append(result, normalizeNumbers(v))
	}
	if result == nil {
		result = []interface{}{}
	}
	return result, nil
}

func serializeJSONL(v interface{}) ([]byte, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("JSONL requires an array of objects")
	}
	var buf bytes.Buffer
	for _, item := range arr {
		b, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		buf.Write(b)
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// --- Env ---

func parseEnv(data []byte) (interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
			(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
			val = val[1 : len(val)-1]
		}
		result[key] = val
	}
	return result, nil
}

func serializeEnv(v interface{}) ([]byte, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("env format requires a flat key-value object")
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(strings.ToUpper(k))
		buf.WriteString("=")
		buf.WriteString(toString(m[k]))
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// toString converts any value to a string representation.
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case bool:
		return strconv.FormatBool(val)
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}
