package converter

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// parseTOMLSimple is a minimal TOML parser that handles the common subset:
// - key = value pairs
// - [section] and [section.subsection] headers
// - [[array.of.tables]] array tables
// - Strings (basic and literal), integers, floats, booleans
// - Inline tables {a = 1, b = 2}
// - Arrays [1, 2, 3]

func parseTOMLSimple(data []byte) (interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(string(data), "\n")

	currentTable := result

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Array of tables: [[section]]
		if strings.HasPrefix(line, "[[") && strings.HasSuffix(line, "]]") {
			path := strings.TrimSpace(line[2 : len(line)-2])
			keys := splitTOMLKey(path)
			arr, err := getOrCreateArrayTable(currentTable, result, keys)
			if err != nil {
				return nil, err
			}
			newTable := make(map[string]interface{})
			arr = append(arr, newTable)
			setNestedValue(result, keys, arr)
			currentTable = newTable
			continue
		}

		// Table header: [section]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			path := strings.TrimSpace(line[1 : len(line)-1])
			keys := splitTOMLKey(path)
			tbl, err := getOrCreateTable(result, keys)
			if err != nil {
				return nil, err
			}
			currentTable = tbl
			continue
		}

		// Key = value
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:eqIdx])
		key = unquoteTOML(key)
		valueStr := strings.TrimSpace(line[eqIdx+1:])
		valueStr = stripTOMLComment(valueStr)

		// Handle multi-line strings and arrays
		if strings.HasPrefix(valueStr, `"""`) {
			value, nextI, err := parseTOMLMultilineString(lines, i, valueStr)
			if err != nil {
				return nil, err
			}
			currentTable[key] = value
			i = nextI
			continue
		}

		val, err := parseTOMLValue(valueStr)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", i+1, err)
		}
		currentTable[key] = val
	}

	return result, nil
}

func splitTOMLKey(path string) []string {
	// Split by . but respect quotes
	var keys []string
	inSingle := false
	inDouble := false
	start := 0
	for i, ch := range path {
		switch ch {
		case '\'':
			inSingle = !inSingle
		case '"':
			inDouble = !inDouble
		case '.':
			if !inSingle && !inDouble {
				keys = append(keys, unquoteTOML(strings.TrimSpace(path[start:i])))
				start = i + 1
			}
		}
	}
	keys = append(keys, unquoteTOML(strings.TrimSpace(path[start:])))
	return keys
}

func unquoteTOML(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func stripTOMLComment(s string) string {
	inSingle := false
	inDouble := false
	for i, ch := range s {
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
				return strings.TrimSpace(s[:i])
			}
		}
	}
	return s
}

func parseTOMLValue(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}

	// Boolean
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}

	// Integer with optional underscores
	cleanInt := strings.ReplaceAll(s, "_", "")
	if i, err := strconv.ParseInt(cleanInt, 10, 64); err == nil {
		return i, nil
	}

	// Float
	cleanFloat := strings.ReplaceAll(s, "_", "")
	if f, err := strconv.ParseFloat(cleanFloat, 64); err == nil {
		return f, nil
	}

	// Basic string
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1], nil
	}

	// Literal string
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1], nil
	}

	// Inline table
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return parseTOMLInlineTable(s)
	}

	// Array
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return parseTOMLArray(s)
	}

	// Fallback: treat as string
	return s, nil
}

func parseTOMLInlineTable(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	s = s[1 : len(s)-1]
	result := make(map[string]interface{})
	if strings.TrimSpace(s) == "" {
		return result, nil
	}

	pairs := splitFlow(s, ',')
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		eqIdx := strings.Index(pair, "=")
		if eqIdx == -1 {
			continue
		}
		key := unquoteTOML(strings.TrimSpace(pair[:eqIdx]))
		val, err := parseTOMLValue(strings.TrimSpace(pair[eqIdx+1:]))
		if err != nil {
			return nil, err
		}
		result[key] = val
	}
	return result, nil
}

func parseTOMLArray(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	s = s[1 : len(s)-1]
	if strings.TrimSpace(s) == "" {
		return []interface{}{}, nil
	}

	items := splitFlow(s, ',')
	result := make([]interface{}, 0, len(items))
	for _, item := range items {
		val, err := parseTOMLValue(strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func parseTOMLMultilineString(lines []string, startI int, firstLine string) (string, int, error) {
	// Find the closing """
	content := firstLine[3:] // skip opening """

	// Check if closing is on same line
	if idx := strings.Index(content, `"""`); idx >= 0 {
		return strings.TrimSpace(content[:idx]), startI, nil
	}

	var buf bytes.Buffer
	buf.WriteString(content)
	i := startI + 1
	for i < len(lines) {
		if idx := strings.Index(lines[i], `"""`); idx >= 0 {
			buf.WriteString(lines[i][:idx])
			break
		}
		buf.WriteString(lines[i])
		buf.WriteString("\n")
		i++
	}
	return strings.TrimSpace(buf.String()), i, nil
}

func getOrCreateTable(root map[string]interface{}, keys []string) (map[string]interface{}, error) {
	current := root
	for _, key := range keys {
		existing, ok := current[key]
		if !ok {
			newTable := make(map[string]interface{})
			current[key] = newTable
			current = newTable
		} else if m, ok := existing.(map[string]interface{}); ok {
			current = m
		} else if arr, ok := existing.([]interface{}); ok {
			// Array of tables - use last element
			if len(arr) > 0 {
				if m, ok := arr[len(arr)-1].(map[string]interface{}); ok {
					current = m
				} else {
					return nil, fmt.Errorf("cannot navigate into key %s", key)
				}
			} else {
				newTable := make(map[string]interface{})
				current[key] = []interface{}{newTable}
				current = newTable
			}
		} else {
			return nil, fmt.Errorf("key %s already exists with non-table value", key)
		}
	}
	return current, nil
}

func getOrCreateArrayTable(currentTable, root map[string]interface{}, keys []string) ([]interface{}, error) {
	// Navigate to parent
	parent := root
	for i := 0; i < len(keys)-1; i++ {
		existing, ok := parent[keys[i]]
		if !ok {
			newTable := make(map[string]interface{})
			parent[keys[i]] = newTable
			parent = newTable
		} else if m, ok := existing.(map[string]interface{}); ok {
			parent = m
		} else {
			return nil, fmt.Errorf("cannot navigate into key %s", keys[i])
		}
	}

	lastKey := keys[len(keys)-1]
	existing, ok := parent[lastKey]
	if !ok {
		arr := []interface{}{}
		parent[lastKey] = arr
		return arr, nil
	}
	if arr, ok := existing.([]interface{}); ok {
		return arr, nil
	}
	return nil, fmt.Errorf("key %s already exists with non-array value", lastKey)
}

func setNestedValue(root map[string]interface{}, keys []string, value interface{}) {
	current := root
	for i := 0; i < len(keys)-1; i++ {
		existing, ok := current[keys[i]]
		if !ok {
			newTable := make(map[string]interface{})
			current[keys[i]] = newTable
			current = newTable
		} else if m, ok := existing.(map[string]interface{}); ok {
			current = m
		}
	}
	current[keys[len(keys)-1]] = value
}

// --- TOML Serializer ---

func serializeTOMLSimple(v interface{}) ([]byte, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("TOML requires a top-level object (map)")
	}

	var buf bytes.Buffer
	var headerBuf bytes.Buffer

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := m[key]
		switch v := val.(type) {
		case map[string]interface{}:
			writeTOMLTable(&buf, &headerBuf, key, v)
		case []interface{}:
			if isArrayOfTables(v) {
				writeTOMLArrayTable(&buf, key, v)
			} else {
				writeTOMLKeyValue(&headerBuf, key, v)
			}
		default:
			writeTOMLKeyValue(&headerBuf, key, v)
		}
	}

	// Header (top-level keys) first, then tables
	result := bytes.Buffer{}
	result.Write(headerBuf.Bytes())
	result.Write(buf.Bytes())
	return result.Bytes(), nil
}

func writeTOMLKeyValue(buf *bytes.Buffer, key string, val interface{}) {
	buf.WriteString(key)
	buf.WriteString(" = ")
	writeTOMLValue(buf, val)
	buf.WriteString("\n")
}

func writeTOMLValue(buf *bytes.Buffer, val interface{}) {
	switch v := val.(type) {
	case nil:
		buf.WriteString("\"\"")
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case int:
		fmt.Fprintf(buf, "%d", v)
	case int64:
		fmt.Fprintf(buf, "%d", v)
	case float64:
		if v == float64(int64(v)) {
			fmt.Fprintf(buf, "%d", int64(v))
		} else {
			fmt.Fprintf(buf, "%v", v)
		}
	case string:
		buf.WriteString(`"`)
		buf.WriteString(escapeTOMLString(v))
		buf.WriteString(`"`)
	case []interface{}:
		buf.WriteString("[")
		for i, item := range v {
			if i > 0 {
				buf.WriteString(", ")
			}
			writeTOMLValue(buf, item)
		}
		buf.WriteString("]")
	case map[string]interface{}:
		buf.WriteString("{")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(k)
			buf.WriteString(" = ")
			writeTOMLValue(buf, v[k])
		}
		buf.WriteString("}")
	default:
		fmt.Fprintf(buf, "%v", v)
	}
}

func writeTOMLTable(buf, headerBuf *bytes.Buffer, prefix string, m map[string]interface{}) {
	// Write scalar/array values first
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf.WriteString("\n[")
	buf.WriteString(prefix)
	buf.WriteString("]\n")

	for _, key := range keys {
		val := m[key]
		switch v := val.(type) {
		case map[string]interface{}:
			writeTOMLTable(buf, headerBuf, prefix+"."+key, v)
		case []interface{}:
			if isArrayOfTables(v) {
				writeTOMLArrayTable(buf, prefix+"."+key, v)
			} else {
				writeTOMLKeyValue(buf, key, v)
			}
		default:
			writeTOMLKeyValue(buf, key, v)
		}
	}
}

func writeTOMLArrayTable(buf *bytes.Buffer, prefix string, arr []interface{}) {
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		buf.WriteString("\n[[")
		buf.WriteString(prefix)
		buf.WriteString("]]\n")

		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			val := m[key]
			switch v := val.(type) {
			case map[string]interface{}:
				writeTOMLTable(buf, buf, prefix+"."+key, v)
			case []interface{}:
				if isArrayOfTables(v) {
					writeTOMLArrayTable(buf, prefix+"."+key, v)
				} else {
					writeTOMLKeyValue(buf, key, v)
				}
			default:
				writeTOMLKeyValue(buf, key, v)
			}
		}
	}
}

func isArrayOfTables(arr []interface{}) bool {
	for _, item := range arr {
		if _, ok := item.(map[string]interface{}); !ok {
			return false
		}
	}
	return len(arr) > 0
}

func escapeTOMLString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
