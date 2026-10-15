package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"core/app/constants"
)

// Adapter 的两个字符串参数是渠道企业/员工身份，记录 ID 必须从请求体取得。
func bindActionID(params json.RawMessage) (string, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.ID) == "" {
		return "", errors.New("id 不能为空")
	}
	return req.ID, nil
}

var formTimeType = reflect.TypeOf(time.Time{})

// 日期控件提交本地日期/时间，Go time.Time 默认只接受 RFC3339。
// 仅规范目标类型中的时间字段，保留生日、期次、编号等字符串原样。
func normalizeFormTimes(raw json.RawMessage, typ reflect.Type) (json.RawMessage, error) {
	if typ == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return raw, nil
	}
	nullable := typ.Kind() == reflect.Pointer
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == formTimeType {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return raw, nil
		}
		if value == "" && nullable {
			return json.RawMessage("null"), nil
		}
		for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
			if parsed, err := time.ParseInLocation(layout, value, constants.PRCLocation); err == nil {
				return json.Marshal(parsed)
			}
		}
		return raw, nil // RFC3339 或非法日期交给标准库严格校验。
	}
	switch typ.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return raw, err
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" && field.Anonymous {
				embedded, err := normalizeFormTimes(raw, field.Type)
				if err != nil {
					return raw, err
				}
				var normalized map[string]json.RawMessage
				if err := json.Unmarshal(embedded, &normalized); err != nil {
					return raw, err
				}
				// 仅合并被嵌入类型实际转换的字段，避免覆盖外层已转换的时间。
				var original map[string]json.RawMessage
				_ = json.Unmarshal(raw, &original)
				for k, value := range normalized {
					if !bytes.Equal(value, original[k]) {
						fields[k] = value
					}
				}
				continue
			}
			if name == "" {
				name = field.Name
			}
			value, ok := fields[name]
			if !ok {
				continue
			}
			normalized, err := normalizeFormTimes(value, field.Type)
			if err != nil {
				return raw, err
			}
			fields[name] = normalized
		}
		return json.Marshal(fields)
	case reflect.Slice, reflect.Array:
		if typ.Elem().Kind() == reflect.Uint8 {
			return raw, nil
		}
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return raw, err
		}
		for i, value := range values {
			normalized, err := normalizeFormTimes(value, typ.Elem())
			if err != nil {
				return raw, err
			}
			values[i] = normalized
		}
		return json.Marshal(values)
	default:
		return raw, nil
	}
}
