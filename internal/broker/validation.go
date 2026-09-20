package broker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
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

// validateResult enforces the wire schema at the last boundary before a
// handler result is encoded. Keeping this check in the dispatcher means every
// connection, including production and test transports, receives the same
// fail-closed behavior.
func validateResult(method string, result any) *jsonrpc.RPCError {
	raw, err := json.Marshal(result)
	if err != nil {
		return invalidResultError("invalid result", "result")
	}
	switch method {
	case "health":
		fields, rpcErr := resultObject(raw, "result", nil, []string{"ok"})
		if rpcErr != nil {
			return rpcErr
		}
		return resultBool(fields, "result.ok", "ok")
	case "execution.start":
		fields, rpcErr := resultObject(raw, "result", nil, []string{"execution_id"})
		if rpcErr != nil {
			return rpcErr
		}
		return resultString(fields, "result.execution_id", "execution_id", true)
	case "execution.status":
		fields, rpcErr := resultObject(raw, "result", nil, []string{"status", "steps"})
		if rpcErr != nil {
			return rpcErr
		}
		if rpcErr := resultEnum(fields, "result.status", "status", "pending", "running", "completed", "failed"); rpcErr != nil {
			return rpcErr
		}
		steps, rpcErr := resultArray(fields, "result.steps", "steps")
		if rpcErr != nil {
			return rpcErr
		}
		for i, rawStep := range steps {
			path := "result.steps[" + strconv.Itoa(i) + "]"
			step, rpcErr := resultObject(rawStep, path, nil, []string{"id", "status"})
			if rpcErr != nil {
				return rpcErr
			}
			if rpcErr := resultString(step, path+".id", "id", true); rpcErr != nil {
				return rpcErr
			}
			if rpcErr := resultEnum(step, path+".status", "status", "pending", "running", "completed", "failed", "skipped"); rpcErr != nil {
				return rpcErr
			}
		}
		return nil
	case "step.run":
		fields, rpcErr := resultObject(raw, "result", nil, []string{"attempt_id", "next_cursor"})
		if rpcErr != nil {
			return rpcErr
		}
		if rpcErr := resultString(fields, "result.attempt_id", "attempt_id", true); rpcErr != nil {
			return rpcErr
		}
		return resultNonNegativeInt(fields, "result.next_cursor", "next_cursor")
	case "step.events":
		fields, rpcErr := resultObject(raw, "result", nil, []string{"events", "next_cursor"})
		if rpcErr != nil {
			return rpcErr
		}
		events, rpcErr := resultArray(fields, "result.events", "events")
		if rpcErr != nil {
			return rpcErr
		}
		if rpcErr := resultNonNegativeInt(fields, "result.next_cursor", "next_cursor"); rpcErr != nil {
			return rpcErr
		}
		for i, rawEvent := range events {
			path := "result.events[" + strconv.Itoa(i) + "]"
			event, rpcErr := resultObject(rawEvent, path, []string{"payload", "payload_ref"}, []string{"cursor", "event_type"})
			if rpcErr != nil {
				return rpcErr
			}
			if rpcErr := resultNonNegativeInt(event, path+".cursor", "cursor"); rpcErr != nil {
				return rpcErr
			}
			if rpcErr := resultString(event, path+".event_type", "event_type", true); rpcErr != nil {
				return rpcErr
			}
			payload, hasPayload := event["payload"]
			payloadRef, hasPayloadRef := event["payload_ref"]
			if hasPayload && hasPayloadRef {
				return invalidResultError("invalid value", path+".payload")
			}
			if hasPayload {
				if rpcErr := resultStringValue(payload, path+".payload", false); rpcErr != nil {
					return rpcErr
				}
				var value string
				_ = json.Unmarshal(payload, &value)
				if len([]byte(value)) > 16*1024 {
					return invalidResultError("value exceeds limit", path+".payload")
				}
			}
			if hasPayloadRef {
				if rpcErr := resultStringValue(payloadRef, path+".payload_ref", true); rpcErr != nil {
					return rpcErr
				}
			}
		}
		return nil
	case "step.approve":
		fields, rpcErr := resultObject(raw, "result", nil, []string{"resolved"})
		if rpcErr != nil {
			return rpcErr
		}
		return resultBool(fields, "result.resolved", "resolved")
	case "step.cancel":
		_, rpcErr := resultObject(raw, "result", nil, nil)
		return rpcErr
	case "step.reopen":
		fields, rpcErr := resultObject(raw, "result", nil, []string{"invalidated"})
		if rpcErr != nil {
			return rpcErr
		}
		invalidated, rpcErr := resultArray(fields, "result.invalidated", "invalidated")
		if rpcErr != nil {
			return rpcErr
		}
		for i, value := range invalidated {
			if rpcErr := resultStringValue(value, "result.invalidated["+strconv.Itoa(i)+"]", true); rpcErr != nil {
				return rpcErr
			}
		}
	}
	return nil
}

func invalidResultError(message, field string) *jsonrpc.RPCError {
	return &jsonrpc.RPCError{Code: jsonrpc.InvalidParamsCode, Message: message, Data: map[string]string{"field": field}}
}

func resultObject(raw json.RawMessage, path string, optional, required []string) (map[string]json.RawMessage, *jsonrpc.RPCError) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, invalidResultError("invalid value", path)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, invalidResultError("invalid value", path)
	}
	allowed := make(map[string]bool, len(optional)+len(required))
	for _, field := range optional {
		allowed[field] = true
	}
	for _, field := range required {
		allowed[field] = true
	}
	unknown := make([]string, 0)
	for field := range fields {
		if !allowed[field] {
			unknown = append(unknown, field)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, invalidResultError("unknown field", path+"."+unknown[0])
	}
	for _, field := range required {
		value, ok := fields[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, invalidResultError("missing field", path+"."+field)
		}
	}
	return fields, nil
}

func resultString(fields map[string]json.RawMessage, path, field string, nonEmpty bool) *jsonrpc.RPCError {
	return resultStringValue(fields[field], path, nonEmpty)
}

func resultStringValue(raw json.RawMessage, path string, nonEmpty bool) *jsonrpc.RPCError {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return invalidResultError("invalid value", path)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || (nonEmpty && value == "") {
		return invalidResultError("invalid value", path)
	}
	return nil
}

func resultBool(fields map[string]json.RawMessage, path, field string) *jsonrpc.RPCError {
	var value bool
	if err := json.Unmarshal(fields[field], &value); err != nil {
		return invalidResultError("invalid value", path)
	}
	return nil
}

func resultEnum(fields map[string]json.RawMessage, path, field string, allowed ...string) *jsonrpc.RPCError {
	var value string
	if err := json.Unmarshal(fields[field], &value); err != nil {
		return invalidResultError("invalid value", path)
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return invalidResultError("invalid enum", path)
}

func resultArray(fields map[string]json.RawMessage, path, field string) ([]json.RawMessage, *jsonrpc.RPCError) {
	var values []json.RawMessage
	if err := json.Unmarshal(fields[field], &values); err != nil || values == nil {
		return nil, invalidResultError("invalid value", path)
	}
	return values, nil
}

func resultNonNegativeInt(fields map[string]json.RawMessage, path, field string) *jsonrpc.RPCError {
	var value int
	if err := json.Unmarshal(fields[field], &value); err != nil || value < 0 {
		return invalidResultError("invalid value", path)
	}
	return nil
}
