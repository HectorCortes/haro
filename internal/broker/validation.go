package broker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

// ParamError describes a deterministic boundary-validation failure.
type ParamError struct {
	Message string
	Field   string
	Detail  string
}

func (e *ParamError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s at %s", e.Message, e.Field)
}

// DecodeParams strictly decodes an object into dst and requires the named
// fields. It is deliberately side-effect free and is called before handlers
// enter a store transaction.
func DecodeParams(raw json.RawMessage, dst any, required ...string) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return &ParamError{Message: "params must be an object", Field: "params"}
	}
	frame := json.NewDecoder(bytes.NewReader(trimmed))
	var object json.RawMessage
	if err := frame.Decode(&object); err != nil {
		return &ParamError{Message: "invalid params", Field: "params", Detail: err.Error()}
	}
	var extra any
	if err := frame.Decode(&extra); err != io.EOF {
		if err == nil {
			return &ParamError{Message: "trailing data", Field: "params"}
		}
		return &ParamError{Message: "trailing data", Field: "params", Detail: err.Error()}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(object, &fields); err != nil {
		return &ParamError{Message: "invalid params", Field: "params", Detail: err.Error()}
	}
	dec := json.NewDecoder(bytes.NewReader(object))
	dec.DisallowUnknownFields()
	value := reflect.ValueOf(dst)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return &ParamError{Message: "invalid destination", Field: "params"}
	}
	target := reflect.New(value.Elem().Type())
	if err := dec.Decode(target.Interface()); err != nil {
		field := "params"
		message := "invalid params"
		if strings.Contains(err.Error(), "unknown field") {
			message = "unknown field"
			field = quotedField(err.Error())
		} else if typeErr, ok := err.(*json.UnmarshalTypeError); ok && typeErr.Field != "" {
			field = typeErr.Field
			message = "invalid value"
		}
		return &ParamError{Message: message, Field: field, Detail: err.Error()}
	}
	value.Elem().Set(target.Elem())
	for _, field := range required {
		value, ok := fields[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return &ParamError{Message: "missing field", Field: field}
		}
	}
	return nil
}

// ValidateEnum returns a boundary error when value is not one of allowed.
func ValidateEnum(value, field string, allowed ...string) error {
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return &ParamError{Message: "invalid enum", Field: field, Detail: value}
}

func quotedField(message string) string {
	start := strings.IndexByte(message, '"')
	if start < 0 {
		return "params"
	}
	end := strings.IndexByte(message[start+1:], '"')
	if end < 0 {
		return "params"
	}
	return message[start+1 : start+1+end]
}

func invalidParamsError(err error) *jsonrpc.RPCError {
	data := map[string]string{}
	if ve, ok := err.(*ParamError); ok {
		if ve.Field != "" {
			data["field"] = ve.Field
		}
		if ve.Detail != "" {
			data["detail"] = ve.Detail
		}
		return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: ve.Message, Data: data}
	}
	return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: "invalid params", Data: data}
}
