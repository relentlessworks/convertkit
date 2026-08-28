package converter

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// parseXML converts XML to a Go map. The root element is unwrapped,
// so the result is the content of the root element as a map.
// Attributes are stored with a leading @ prefix.
// Text content is stored under #text key when mixed with children.

func parseXMLSimple(data []byte) (interface{}, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false

	// Skip to first start element
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return make(map[string]interface{}), nil
		}
		if err != nil {
			return nil, err
		}
		if se, ok := token.(xml.StartElement); ok {
			result, err := parseXMLElement(decoder, se)
			if err != nil {
				return nil, err
			}
			// Unwrap root element
			if m, ok := result.(map[string]interface{}); ok {
				// If root has a single child that's a map, return that
				return m, nil
			}
			return result, nil
		}
	}
}

func parseXMLElement(decoder *xml.Decoder, se xml.StartElement) (interface{}, error) {
	attrs := make(map[string]interface{})
	for _, attr := range se.Attr {
		attrs["@"+attr.Name.Local] = attr.Value
	}

	var textContent strings.Builder
	children := make(map[string]interface{})
	childOrder := []string{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			childVal, err := parseXMLElement(decoder, t)
			if err != nil {
				return nil, err
			}
			name := t.Name.Local
			if existing, ok := children[name]; ok {
				// Convert to array
				if arr, ok := existing.([]interface{}); ok {
					children[name] = append(arr, childVal)
				} else {
					children[name] = []interface{}{existing, childVal}
				}
			} else {
				children[name] = childVal
				childOrder = append(childOrder, name)
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				textContent.WriteString(text)
			}
		case xml.EndElement:
			// Build result
			text := textContent.String()
			if len(children) == 0 && len(attrs) == 0 {
				if text == "" {
					return nil, nil
				}
				return text, nil
			}

			result := make(map[string]interface{})
			for k, v := range attrs {
				result[k] = v
			}
			for k, v := range children {
				result[k] = v
			}
			if text != "" {
				result["#text"] = text
			}
			return result, nil
		}
	}

	return nil, nil
}

// serializeXML converts a Go interface{} to XML.
// The input should be a map. The root element is <root>.

func serializeXMLSimple(v interface{}) ([]byte, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("XML requires a map/object input")
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString("<root>\n")
	if err := writeXMLChildren(&buf, m, 1); err != nil {
		return nil, err
	}
	buf.WriteString("</root>\n")
	return buf.Bytes(), nil
}

func writeXMLChildren(buf *bytes.Buffer, m map[string]interface{}, indent int) error {
	prefix := strings.Repeat("  ", indent)

	keys := make([]string, 0, len(m))
	for k := range m {
		if !strings.HasPrefix(k, "@") && k != "#text" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := m[key]
		writeXMLElement(buf, key, val, prefix, indent)
	}

	// Text content
	if text, ok := m["#text"]; ok {
		buf.WriteString(escapeXMLText(toString(text)))
	}

	return nil
}

func writeXMLElement(buf *bytes.Buffer, name string, val interface{}, prefix string, indent int) {
	switch v := val.(type) {
	case nil:
		fmt.Fprintf(buf, "%s<%s/>\n", prefix, name)
	case string:
		fmt.Fprintf(buf, "%s<%s>%s</%s>\n", prefix, name, escapeXMLText(v), name)
	case bool:
		fmt.Fprintf(buf, "%s<%s>%t</%s>\n", prefix, name, v, name)
	case int, int64, float64:
		fmt.Fprintf(buf, "%s<%s>%v</%s>\n", prefix, name, v, name)
	case map[string]interface{}:
		// Extract attributes
		attrs := []string{}
		attrKeys := make([]string, 0)
		for k := range v {
			if strings.HasPrefix(k, "@") {
				attrKeys = append(attrKeys, k)
			}
		}
		sort.Strings(attrKeys)
		for _, k := range attrKeys {
			attrs = append(attrs, fmt.Sprintf(` %s="%s"`, k[1:], escapeXMLAttr(toString(v[k]))))
		}
		attrStr := strings.Join(attrs, "")

		// Check if it has children or just text
		hasChildren := false
		for k := range v {
			if !strings.HasPrefix(k, "@") && k != "#text" {
				hasChildren = true
				break
			}
		}

		if hasChildren {
			fmt.Fprintf(buf, "%s<%s%s>\n", prefix, name, attrStr)
			writeXMLChildren(buf, v, indent+1)
			fmt.Fprintf(buf, "%s</%s>\n", prefix, name)
		} else if text, ok := v["#text"]; ok {
			fmt.Fprintf(buf, "%s<%s%s>%s</%s>\n", prefix, name, attrStr, escapeXMLText(toString(text)), name)
		} else {
			fmt.Fprintf(buf, "%s<%s%s/>\n", prefix, name, attrStr)
		}
	case []interface{}:
		for _, item := range v {
			writeXMLElement(buf, name, item, prefix, indent)
		}
	default:
		fmt.Fprintf(buf, "%s<%s>%v</%s>\n", prefix, name, v, name)
	}
}

func escapeXMLText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeXMLAttr(s string) string {
	s = escapeXMLText(s)
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
