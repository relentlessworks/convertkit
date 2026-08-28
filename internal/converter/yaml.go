package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// parseYAMLSimple is a minimal YAML parser that handles the most common subset:
// - key: value pairs
// - nested maps via indentation
// - lists with - prefix
// - scalars: strings, numbers, booleans, null
// - flow style: {a: 1, b: 2} and [1, 2, 3]
// This is NOT a full YAML parser but covers the vast majority of config files.

func parseYAMLSimple(data []byte) (interface{}, error) {
	lines := strings.Split(string(data), "\n")
	// Remove BOM if present
	if len(lines) > 0 && strings.HasPrefix(lines[0], "\ufeff") {
		lines[0] = strings.TrimPrefix(lines[0], "\ufeff")
	}
	// Remove document markers
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" || trimmed == "..." {
			continue
		}
		filtered = append(filtered, line)
	}

	parser := &yamlParser{lines: filtered, pos: 0}
	return parser.parseValue(0)
}

type yamlParser struct {
	lines []string
	pos   int
}

func (p *yamlParser) peek() (string, bool) {
	if p.pos >= len(p.lines) {
		return "", false
	}
	return p.lines[p.pos], true
}

func (p *yamlParser) next() (string, bool) {
	if p.pos >= len(p.lines) {
		return "", false
	}
	line := p.lines[p.pos]
	p.pos++
	return line, true
}

func (p *yamlParser) skipBlank() {
	for {
		line, ok := p.peek()
		if !ok {
			return
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			p.pos++
			continue
		}
		return
	}
}

func indentLevel(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 8
		} else {
			break
		}
	}
	return count
}

func (p *yamlParser) parseValue(minIndent int) (interface{}, error) {
	p.skipBlank()
	line, ok := p.peek()
	if !ok {
		return nil, nil
	}

	indent := indentLevel(line)
	if indent < minIndent {
		return nil, nil
	}

	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
		return p.parseList(indent)
	}

	if strings.HasPrefix(trimmed, "{") {
		p.next()
		return parseFlowMap(trimmed)
	}
	if strings.HasPrefix(trimmed, "[") {
		p.next()
		return parseFlowList(trimmed)
	}

	return p.parseMap(indent)
}

func (p *yamlParser) parseMap(indent int) (interface{}, error) {
	result := make(map[string]interface{})

	for {
		p.skipBlank()
		line, ok := p.peek()
		if !ok {
			break
		}

		currentIndent := indentLevel(line)
		if currentIndent < indent {
			break
		}
		if currentIndent > indent {
			p.pos++
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			break
		}

		colonIdx := findColon(trimmed)
		if colonIdx == -1 {
			p.pos++
			continue
		}

		key := unquote(strings.TrimSpace(trimmed[:colonIdx]))
		valuePart := strings.TrimSpace(trimmed[colonIdx+1:])
		valuePart = stripInlineComment(valuePart)

		p.next()

		if valuePart == "" {
			val, err := p.parseValue(indent + 1)
			if err != nil {
				return nil, err
			}
			result[key] = val
		} else {
			result[key] = parseScalar(valuePart)
		}
	}

	return result, nil
}

func (p *yamlParser) parseList(indent int) (interface{}, error) {
	var result []interface{}

	for {
		p.skipBlank()
		line, ok := p.peek()
		if !ok {
			break
		}

		currentIndent := indentLevel(line)
		if currentIndent < indent {
			break
		}

		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") && trimmed != "-" {
			break
		}

		p.next()

		var content string
		if trimmed == "-" {
			content = ""
		} else {
			content = strings.TrimSpace(trimmed[2:])
		}

		content = stripInlineComment(content)

		if content == "" {
			val, err := p.parseValue(indent + 2)
			if err != nil {
				return nil, err
			}
			result = append(result, val)
		} else {
			colonIdx := findColon(content)
			if colonIdx != -1 {
				key := unquote(strings.TrimSpace(content[:colonIdx]))
				valuePart := strings.TrimSpace(content[colonIdx+1:])
				valuePart = stripInlineComment(valuePart)

				item := make(map[string]interface{})
				if valuePart == "" {
					val, err := p.parseValue(indent + 4)
					if err != nil {
						return nil, err
					}
					item[key] = val
				} else {
					item[key] = parseScalar(valuePart)
				}

				contentIndent := indent + 2
				for {
					p.skipBlank()
					nextLine, ok := p.peek()
					if !ok {
						break
					}
					nextIndent := indentLevel(nextLine)
					nextTrimmed := strings.TrimSpace(nextLine)
					if nextIndent < contentIndent || strings.HasPrefix(nextTrimmed, "- ") {
						break
					}
					if nextIndent > contentIndent {
						break
					}

					nextColon := findColon(nextTrimmed)
					if nextColon == -1 {
						break
					}

					nextKey := unquote(strings.TrimSpace(nextTrimmed[:nextColon]))
					nextValue := strings.TrimSpace(nextTrimmed[nextColon+1:])
					nextValue = stripInlineComment(nextValue)
					p.next()

					if nextValue == "" {
						val, err := p.parseValue(contentIndent + 1)
						if err != nil {
							return nil, err
						}
						item[nextKey] = val
					} else {
						item[nextKey] = parseScalar(nextValue)
					}
				}

				result = append(result, item)
			} else {
				result = append(result, parseScalar(content))
			}
		}
	}

	if result == nil {
		result = []interface{}{}
	}
	return result, nil
}

func findColon(s string) int {
	inSingle := false
	inDouble := false
	depth := 0
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
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ':':
			if !inSingle && !inDouble && depth == 0 {
				if i+1 >= len(s) || s[i+1] == ' ' || s[i+1] == '\t' {
					return i
				}
			}
		}
	}
	return -1
}

func stripInlineComment(s string) string {
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

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseScalar(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var result string
		if err := json.Unmarshal([]byte(s), &result); err == nil {
			return result
		}
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}

	if strings.HasPrefix(s, "{") {
		v, _ := parseFlowMap(s)
		return v
	}
	if strings.HasPrefix(s, "[") {
		v, _ := parseFlowList(s)
		return v
	}

	switch s {
	case "null", "~", "Null", "NULL":
		return nil
	case "true", "True", "TRUE":
		return true
	case "false", "False", "FALSE":
		return false
	}

	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return s
}

func parseFlowMap(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("invalid flow map: %s", s)
	}
	s = s[1 : len(s)-1]
	result := make(map[string]interface{})
	if strings.TrimSpace(s) == "" {
		return result, nil
	}

	pairs := splitFlow(s, ',')
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		colonIdx := strings.Index(pair, ":")
		if colonIdx == -1 {
			continue
		}
		key := unquote(strings.TrimSpace(pair[:colonIdx]))
		val := strings.TrimSpace(pair[colonIdx+1:])
		result[key] = parseScalar(val)
	}
	return result, nil
}

func parseFlowList(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("invalid flow list: %s", s)
	}
	s = s[1 : len(s)-1]
	if strings.TrimSpace(s) == "" {
		return []interface{}{}, nil
	}

	items := splitFlow(s, ',')
	result := make([]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, parseScalar(strings.TrimSpace(item)))
	}
	return result, nil
}

func splitFlow(s string, delim byte) []string {
	var parts []string
	depth := 0
	inSingle := false
	inDouble := false
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '{', '[':
			if !inSingle && !inDouble {
				depth++
			}
		case '}', ']':
			if !inSingle && !inDouble {
				depth--
			}
		case delim:
			if !inSingle && !inDouble && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// --- YAML Serializer ---

func serializeYAMLSimple(v interface{}, indent int) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeYAML(&buf, v, indent, true); err != nil {
		return nil, err
	}
	output := bytes.TrimRight(buf.Bytes(), "\n")
	if len(output) > 0 {
		output = append(output, '\n')
	}
	return output, nil
}

func writeYAML(buf *bytes.Buffer, v interface{}, indent int, atRoot bool) error {
	prefix := strings.Repeat("  ", indent)

	switch val := v.(type) {
	case nil:
		buf.WriteString("null\n")
	case bool:
		if val {
			buf.WriteString("true\n")
		} else {
			buf.WriteString("false\n")
		}
	case int:
		fmt.Fprintf(buf, "%d\n", val)
	case int64:
		fmt.Fprintf(buf, "%d\n", val)
	case float64:
		if val == float64(int64(val)) {
			fmt.Fprintf(buf, "%d\n", int64(val))
		} else {
			fmt.Fprintf(buf, "%v\n", val)
		}
	case string:
		if needsYAMLQuote(val) {
			b, _ := json.Marshal(val)
			buf.Write(b)
		} else {
			buf.WriteString(val)
		}
		buf.WriteString("\n")
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, k := range keys {
			buf.WriteString(prefix)
			buf.WriteString(k)
			buf.WriteString(":")
			child := val[k]
			switch child.(type) {
			case map[string]interface{}, []interface{}:
				if child == nil {
					buf.WriteString(" null\n")
				} else {
					buf.WriteString("\n")
					if err := writeYAML(buf, child, indent+1, false); err != nil {
						return err
					}
				}
			default:
				buf.WriteString(" ")
				if err := writeYAML(buf, child, indent, false); err != nil {
					return err
				}
			}
		}
	case []interface{}:
		for _, item := range val {
			buf.WriteString(prefix)
			buf.WriteString("- ")
			switch item.(type) {
			case map[string]interface{}:
				m := item.(map[string]interface{})
				mkeys := make([]string, 0, len(m))
				for k := range m {
					mkeys = append(mkeys, k)
				}
				sortStrings(mkeys)
				if len(mkeys) > 0 {
					buf.WriteString(mkeys[0])
					buf.WriteString(":")
					firstVal := m[mkeys[0]]
					switch firstVal.(type) {
					case map[string]interface{}, []interface{}:
						buf.WriteString("\n")
						if err := writeYAML(buf, firstVal, indent+2, false); err != nil {
							return err
						}
					default:
						buf.WriteString(" ")
						if err := writeYAML(buf, firstVal, indent, false); err != nil {
							return err
						}
					}
					for i := 1; i < len(mkeys); i++ {
						buf.WriteString(strings.Repeat("  ", indent+1))
						buf.WriteString(mkeys[i])
						buf.WriteString(":")
						childVal := m[mkeys[i]]
						switch childVal.(type) {
						case map[string]interface{}, []interface{}:
							buf.WriteString("\n")
							if err := writeYAML(buf, childVal, indent+2, false); err != nil {
								return err
							}
						default:
							buf.WriteString(" ")
							if err := writeYAML(buf, childVal, indent, false); err != nil {
								return err
							}
						}
					}
				}
			default:
				if err := writeYAML(buf, item, indent, false); err != nil {
					return err
				}
			}
		}
	default:
		b, _ := json.Marshal(val)
		buf.Write(b)
		buf.WriteString("\n")
	}
	return nil
}

func needsYAMLQuote(s string) bool {
	if s == "" {
		return true
	}
	switch s {
	case "true", "false", "null", "~", "yes", "no", "on", "off",
		"True", "False", "Null", "Yes", "No", "On", "Off",
		"TRUE", "FALSE", "NULL", "YES", "NO", "ON", "OFF":
		return true
	}
	if strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`") {
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return true
	}
	if s != strings.TrimSpace(s) {
		return true
	}
	return false
}
