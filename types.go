package jsonrpc2

import (
	"encoding/json"
	"fmt"
	"io"
)

type jsonRpcErrorCode = int

const (
	ParseError     jsonRpcErrorCode = -32700
	InvalidRequest jsonRpcErrorCode = -32600
	MethodNotFound jsonRpcErrorCode = -32601
	InvalidParams  jsonRpcErrorCode = -32602
	InternalError  jsonRpcErrorCode = -32603
)

var (
	m = map[jsonRpcErrorCode]string{
		ParseError:     "Invalid JSON was received by the server, unable to parse.",
		InvalidRequest: "The JSON sent is not a valid request.",
		MethodNotFound: "The method does not exist",
		InvalidParams:  "invalid method parameters",
		InternalError:  "Internal error processing request.",
	}
)

func NewJsonRpcError(code jsonRpcErrorCode, data any) *JsonRpcError {
	if message, ok := m[code]; ok {
		return &JsonRpcError{
			Code:    code,
			Message: message,
			Data:    data,
		}
	} else {
		message = m[InternalError]
		return &JsonRpcError{
			Code:    InternalError,
			Message: message,
			Data:    nil,
		}
	}
}

func NewJsonRpcErrorCode(message string) (*jsonRpcErrorCode, error) {
	// These codes are reserved for an individual server
	for i := -32000; i >= -32099; i-- {
		if _, ok := m[i]; !ok {
			m[i] = message
			return &i, nil
		}
	}
	return nil, fmt.Errorf("exhaused the number of server defined errors")
}

type JsonRpcError struct {
	Code    jsonRpcErrorCode `json:"code"`
	Message string           `json:"message"`
	Data    any              `json:"data,omitempty"`
}

func (j JsonRpcError) fromMap(_ map[string]any) (bool, error) {
	// This should never be called.
	return true, nil
}

func (j JsonRpcError) fromSlice(_ []any) (bool, error) {
	return false, nil
}

func (j JsonRpcError) Error() string {
	if j.Data == nil {
		return fmt.Sprintf("%v: %s", j.Code, j.Message)
	} else {
		return fmt.Sprintf("%v: %s\n%#v", j.Code, j.Message, j.Data)
	}
}

var _ error = JsonRpcError{}

// A Request to be handled by the json rpc server. Can be a method call (id is set)  or notification (id is nil)
type Request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	// by-position: params MUST be an Array, containing the values in the Server expected order.
	// by-name: params MUST be an Object, with member names that match the Server expected parameter names.
	// The absence of expected names MAY result in an error being generated.
	// The names MUST match exactly, including case, to the method's expected parameters.
	Params any `json:"params,omitempty"`
	// An identifier established by the Client that MUST contain a String, Number, or NULL value if included.
	// If it is not included it is assumed to be a notification.
	// The value SHOULD normally not be Null and Numbers SHOULD NOT contain fractional parts [2]
	// Note: that if we recieve a NULL id we assume it to be nil and will be treated as a notification.
	ID *string `json:"id,omitempty"`
}

var _ MessageMap = &Request{}

type BatchRequest []*Request

func (b BatchRequest) fromMap(_ map[string]any) (bool, error) {
	return false, nil
}

func (b BatchRequest) fromSlice(a []any) (bool, error) {
	var err error
	for _, r := range a {
		if m, ok := r.(map[string]any); ok {
			var o bool
			req := &Request{}
			o, err = req.fromMap(m)
			if !o || err != nil {
				break
			}
			b = append(b, req)
		} else {
			return false, NewJsonRpcError(ParseError, a)
		}
	}
	return false, err
}

var _ MessageMap = BatchRequest{}

type Response struct {
	Version string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`

	Error *JsonRpcError `json:"error,omitempty"`

	//It MUST be the same as the value of the id member in the Request Object.
	//If there was an error in detecting the id in the Request object (e.g. Parse error/Invalid Request), it MUST be Null.
	ID *string `json:"id"`
}

var _ MessageMap = &Response{}

type BatchResponse []*Response

func (b BatchResponse) fromMap(_ map[string]any) (bool, error) {
	return false, nil
}

func (b BatchResponse) fromSlice(a []any) (bool, error) {
	var err error
	for _, r := range a {
		if m, ok := r.(map[string]any); ok {
			var o bool
			res := &Response{}
			o, err = res.fromMap(m)
			if !o || err != nil {
				break
			}
			b = append(b, res)
		} else {
			return false, NewJsonRpcError(ParseError, a)
		}
	}
	return false, err
}

var _ MessageMap = BatchResponse{}

type MessageMap interface {
	fromMap(map[string]any) (bool, error)
	fromSlice([]any) (bool, error)
}

func (r *Request) fromSlice(m []any) (bool, error) {
	return false, nil
}

func (r *Request) fromMap(m map[string]any) (bool, error) {
	if v, ok := m["method"]; ok {
		r.Method, ok = v.(string)
		if !ok {
			return false, NewJsonRpcError(ParseError, m)
		}
	} else {
		// if there is no method it is not a request
		return false, nil
	}
	// id can be nil in the case of a notification.
	if v, ok := m["id"]; ok {
		if v != nil {
			id := parseIdFromAny(v)
			r.ID = id
		}
	}
	// Params can be nil
	if v, ok := m["params"]; ok {
		r.Params = v
	}
	return true, nil
}

func (r *Response) fromSlice(m []any) (bool, error) {
	return false, nil
}

func (r *Response) fromMap(m map[string]any) (bool, error) {
	if v, ok := m["id"]; ok {
		if v != nil {
			id := parseIdFromAny(v)
			r.ID = id
		}
	}
	if v, ok := m["result"]; ok {
		r.Result = v
	}
	if v, ok := m["error"]; ok {
		if v != nil {
			if em, ok := v.(map[string]interface{}); !ok {
				return false, NewJsonRpcError(ParseError, m)
			} else {
				var err error
				r.Error, err = parseJsonRPCError(em)
				if err != nil {
					return false, err
				}
			}
		}
	}
	if r.Result == nil && r.Error == nil {
		return false, NewJsonRpcError(ParseError, m)
	}
	return true, nil
}

func parseIdFromAny(id any) *string {
	switch i := id.(type) {
	case string:
		return &i
	case float64:
		s := fmt.Sprintf("%v", i)
		return &s
	default:
		return nil
	}
}

func parseJsonRPCError(m map[string]any) (*JsonRpcError, error) {
	rpcError := &JsonRpcError{}
	if v, ok := m["code"]; !ok {
		return nil, nil
	} else {
		if c, ok := v.(float64); ok {
			rpcError.Code = int(c)
			if float64(rpcError.Code) != c {
				return nil, NewJsonRpcError(ParseError, m)
			}
		}
	}
	if v, ok := m["message"]; ok {
		if rpcError.Message, ok = v.(string); !ok {
			return nil, NewJsonRpcError(ParseError, m)
		}
	} else {
		return nil, NewJsonRpcError(ParseError, m)
	}
	rpcError.Data = m["data"]
	return rpcError, nil
}

func GetMessage(b []byte) (MessageMap, error) {
	m := map[string]any{}
	if len(b) == 0 {
		return nil, io.EOF
	}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	var version string
	if v, ok := m["jsonrpc"]; !ok || v != "2.0" {
		return nil, NewJsonRpcError(InvalidRequest, m)
	} else if version, ok = v.(string); !ok {
		return nil, NewJsonRpcError(InvalidRequest, m)
	}

	r := &Request{}
	if ok, requestError := r.fromMap(m); ok {
		r.Version = version
		return r, nil
	} else if requestError != nil {
		return nil, requestError
	}
	res := &Response{}
	if ok, responseError := res.fromMap(m); ok {
		res.Version = version
		return res, nil
	} else if responseError != nil {
		return nil, responseError
	}
	return nil, NewJsonRpcError(InvalidRequest, m)
}
