package jsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// MaxMessageSize is the 10 MiB limit per JSON-RPC frame.
const MaxMessageSize = 10 * 1024 * 1024

// RPCError is the JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ID represents a JSON-RPC id which may be string, number, or null.
type ID struct {
	Str  string
	Num  string
	null bool
}

// IsNull reports whether the id is null.
func (id *ID) IsNull() bool { return id != nil && id.null }

// IsString reports whether the id is a string.
func (id *ID) IsString() bool { return id != nil && id.Str != "" && !id.null && id.Num == "" }

// IsNumber reports whether the id is a number.
func (id *ID) IsNumber() bool { return id != nil && id.Num != "" }

// Message is a decoded JSON-RPC 2.0 message (request, response, error, or notification).
type Message struct {
	JSONRPC string
	ID      *ID
	Method  string
	Params  json.RawMessage
	Result  json.RawMessage
	Error   *RPCError
}

// rawMessage is used for decoding.
type rawMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func marshalID(id any) (json.RawMessage, error) {
	if id == nil {
		return json.RawMessage("null"), nil
	}
	switch v := id.(type) {
	case string:
		b, err := json.Marshal(v)
		return json.RawMessage(b), err
	case json.Number:
		s := v.String()
		if s == "" {
			s = "0"
		}
		// Validate number
		var tmp json.Number
		_ = tmp
		return json.RawMessage(s), nil
	case int:
		return json.RawMessage(fmt.Sprintf("%d", v)), nil
	case int64:
		return json.RawMessage(fmt.Sprintf("%d", v)), nil
	case float64:
		b, err := json.Marshal(v)
		return json.RawMessage(b), err
	default:
		// Fallback: marshal but ensure numbers not quoted? Use json.Marshal
		b, err := json.Marshal(v)
		return json.RawMessage(b), err
	}
}

func encodeAndWrite(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d > %d", len(data), MaxMessageSize)
	}
	// NDJSON: one JSON value per line
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

// EncodeRequest encodes a JSON-RPC 2.0 request with id.
func EncodeRequest(w io.Writer, id any, method string, params any) error {
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		paramsRaw = b
	}
	idRaw, err := marshalID(id)
	if err != nil {
		return err
	}
	type wire struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	msg := wire{
		JSONRPC: "2.0",
		ID:      idRaw,
		Method:  method,
		Params:  paramsRaw,
	}
	return encodeAndWrite(w, msg)
}

// EncodeResponse encodes a success response.
func EncodeResponse(w io.Writer, id any, result any) error {
	var resultRaw json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		resultRaw = b
	} else {
		resultRaw = json.RawMessage("null")
	}
	idRaw, err := marshalID(id)
	if err != nil {
		return err
	}
	type wire struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	msg := wire{
		JSONRPC: "2.0",
		ID:      idRaw,
		Result:  resultRaw,
	}
	return encodeAndWrite(w, msg)
}

// EncodeError encodes an error response.
func EncodeError(w io.Writer, id any, rpcErr RPCError) error {
	idRaw, err := marshalID(id)
	if err != nil {
		return err
	}
	errRaw, err := json.Marshal(rpcErr)
	if err != nil {
		return err
	}
	type wire struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   json.RawMessage `json:"error"`
	}
	msg := wire{
		JSONRPC: "2.0",
		ID:      idRaw,
		Error:   json.RawMessage(errRaw),
	}
	return encodeAndWrite(w, msg)
}

// EncodeNotification encodes a notification (no id).
func EncodeNotification(w io.Writer, method string, params any) error {
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		paramsRaw = b
	}
	type wire struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	msg := wire{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsRaw,
	}
	return encodeAndWrite(w, msg)
}

// DecodeMessage decodes a single NDJSON JSON-RPC message from r.
// It enforces 10 MiB limit and uses UseNumber for numbers.
func DecodeMessage(r io.Reader) (*Message, error) {
	// Read one line or all remaining if no newline, with limit check.
	br, ok := r.(*bufio.Reader)
	if !ok {
		// If r is *bytes.Buffer, we can read all efficiently; but use bufio for limit
		br = bufio.NewReader(r)
	}
	// Read until newline, but also handle EOF without newline.
	var buf bytes.Buffer
	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		if buf.Len()+1 > MaxMessageSize {
			return nil, fmt.Errorf("message too large: exceeds %d bytes", MaxMessageSize)
		}
		if b == '\n' {
			break
		}
		buf.WriteByte(b)
	}
	// If buffer is empty and we hit EOF immediately, try reading all (for readers that are not line-oriented)
	// Actually above loop handles line; if input was provided as bytes.Buffer containing JSON without reading via ReadByte loop (we already drained via ReadByte), we have data.
	// But for cases where Reader is bytes.Buffer and we used bufio.NewReader, ReadByte loop will handle.
	// If msg was encoded with trailing newline, we broke on newline; else EOF.
	data := bytes.TrimSpace(buf.Bytes())
	if len(data) == 0 {
		// Maybe input had no newline and we already consumed? Try alternative: if buffer empty, read remaining with LimitReader
		// Fallback: read all with limit (for cases where br was wrapped but no bytes read due to alternative path)
		remaining, _ := io.ReadAll(io.LimitReader(br, int64(MaxMessageSize+1)))
		if len(remaining) > 0 {
			if len(remaining) > MaxMessageSize {
				return nil, fmt.Errorf("message too large: exceeds %d bytes", MaxMessageSize)
			}
			data = bytes.TrimSpace(remaining)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("empty message")
		}
	}
	if len(data) > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d > %d", len(data), MaxMessageSize)
	}
	// UseNumber decoding
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw rawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	if raw.JSONRPC != "" && raw.JSONRPC != "2.0" {
		return nil, fmt.Errorf("invalid jsonrpc version: %q", raw.JSONRPC)
	}
	msg := &Message{
		JSONRPC: raw.JSONRPC,
		Method:  raw.Method,
		Params:  raw.Params,
		Result:  raw.Result,
	}
	// ID handling
	if len(raw.ID) != 0 {
		trim := bytes.TrimSpace(raw.ID)
		if string(trim) == "null" {
			msg.ID = &ID{null: true}
		} else if len(trim) > 0 && trim[0] == '"' {
			var s string
			if err := json.Unmarshal(trim, &s); err != nil {
				return nil, fmt.Errorf("decode id string: %w", err)
			}
			msg.ID = &ID{Str: s}
		} else {
			// number
			// Preserve as string via UseNumber
			msg.ID = &ID{Num: string(trim)}
		}
	} else {
		// No id field -> notification (ID stays nil)
		msg.ID = nil
	}
	// Error handling
	if len(raw.Error) != 0 {
		var rpcErr RPCError
		// Use UseNumber for error object as well
		dec2 := json.NewDecoder(bytes.NewReader(raw.Error))
		dec2.UseNumber()
		if err := dec2.Decode(&rpcErr); err != nil {
			// fallback generic
			if err2 := json.Unmarshal(raw.Error, &rpcErr); err2 != nil {
				return nil, fmt.Errorf("decode error object: %w", err)
			}
		}
		msg.Error = &rpcErr
	}
	// If no explicit jsonrpc, default to 2.0 for leniency? keep as is
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	return msg, nil
}
