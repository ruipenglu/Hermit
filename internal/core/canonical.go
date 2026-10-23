package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// canonicalJSON 生成与旧版 Python json.dumps(value, sort_keys=True) 完全一致的
// 文本：键排序、", "/": " 分隔、非 ASCII 转 \uXXXX。用于跨版本比较内容哈希。
// json.Number（UseNumber 解码）按原文数字输出，与 Python 整数往返一致。
func canonicalJSON(value any) string {
	var builder strings.Builder
	writeCanonical(&builder, value)
	return builder.String()
}

func writeCanonical(builder *strings.Builder, value any) {
	switch typed := value.(type) {
	case nil:
		builder.WriteString("null")
	case bool:
		if typed {
			builder.WriteString("true")
		} else {
			builder.WriteString("false")
		}
	case string:
		writeCanonicalString(builder, typed)
	case json.Number:
		builder.WriteString(typed.String())
	case int:
		builder.WriteString(strconv.Itoa(typed))
	case int64:
		builder.WriteString(strconv.FormatInt(typed, 10))
	case float64:
		builder.WriteString(canonicalFloat(typed))
	case []any:
		builder.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				builder.WriteString(", ")
			}
			writeCanonical(builder, item)
		}
		builder.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		builder.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				builder.WriteString(", ")
			}
			writeCanonicalString(builder, key)
			builder.WriteString(": ")
			writeCanonical(builder, typed[key])
		}
		builder.WriteByte('}')
	default:
		writeCanonicalString(builder, fmt.Sprint(value))
	}
}

// canonicalFloat 对齐 Python repr：整值浮点保留 ".0"。
func canonicalFloat(value float64) string {
	if value == float64(int64(value)) && value < 1e16 && value > -1e16 {
		return strconv.FormatFloat(value, 'f', 1, 64)
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

const canonicalHexDigits = "0123456789abcdef"

func writeCanonicalString(builder *strings.Builder, s string) {
	builder.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		default:
			if c >= 0x20 && c <= 0x7e {
				builder.WriteRune(c)
			} else if c > 0xffff {
				// 基本多文种平面之外按代理对拆分，与 Python ensure_ascii 一致
				code := c - 0x10000
				writeUnicodeEscape(builder, 0xd800+(code>>10))
				writeUnicodeEscape(builder, 0xdc00+(code&0x3ff))
			} else {
				writeUnicodeEscape(builder, c)
			}
		}
	}
	builder.WriteByte('"')
}

func writeUnicodeEscape(builder *strings.Builder, c rune) {
	builder.WriteString(`\u`)
	buffer := [4]byte{}
	digits := uint32(c)
	for i := 3; i >= 0; i-- {
		buffer[i] = canonicalHexDigits[digits&0xf]
		digits >>= 4
	}
	builder.Write(buffer[:])
}

// NormalizeNumbers 把 JSON 解码保留的 json.Number 转为具体数值类型，
// 供 YAML 序列化等不接受 json.Number 的路径使用。
func NormalizeNumbers(value any) any { return normalizeNumbers(value) }

func normalizeNumbers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = normalizeNumbers(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = normalizeNumbers(item)
		}
		return result
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return n
		}
		f, _ := typed.Float64()
		return f
	default:
		return value
	}
}
