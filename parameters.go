package jsonrpc2

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"
)

// TODO: shawn-hurley need to handle custom json marshal for interface types,
// and handle all map cases where not map[string]any

func getParameters(parameters Parameters, requestParams any, log *slog.Logger) (Parameters, *JsonRpcError) {
	newParameters := reflect.New(reflect.TypeOf(parameters).Elem())
	p, ok := newParameters.Interface().(Parameters)
	if !ok {
		log.Error("something is very wrong, can not make parameters of type p")
		return nil, NewJsonRpcError(InternalError, nil)
	}
	p.FromAny(requestParams)
	log.Info(fmt.Sprintf("params: %+v", p))
	return p, nil
}

func GetArgs[T any](params Parameters) (*T, error) {
	typedParams, ok := params.(*TypedParameters[T])
	if !ok {
		return nil, fmt.Errorf("unable to get parameters for type")
	}
	return &typedParams.Args, nil
}

// Parameters allow the user to chose to use Array based parameters or named parameters.
type Parameters interface {
	FromAny(r any) error
}

type TypedParameters[t any] struct {
	Args t
}

func (n *TypedParameters[t]) FromAny(r any) error {
	var zero [0]t
	tt := reflect.TypeOf(zero).Elem()
	var err error
	switch v := r.(type) {
	case map[string]any:
		if !(tt.Kind() == reflect.Struct || (tt.Kind() == reflect.Pointer && tt.Elem().Kind() == reflect.Struct)) {
			return NewJsonRpcError(InvalidRequest, r)
		}
		n.Args, err = n.fromMap(v, tt)
		if err != nil {
			return err
		}
	case []any:
		if tt.Kind() != reflect.Array && tt.Kind() != reflect.Slice {
			return NewJsonRpcError(InvalidRequest, r)
		}
		n.Args, err = n.fromArray(v, tt)
		if err != nil {
			return err
		}
	default:
		return NewJsonRpcError(ParseError, r)
	}
	return nil
}

func (n *TypedParameters[t]) fromMap(m map[string]any, tt reflect.Type) (t, error) {
	returnT := reflect.New(tt).Elem()
	v, err := fromMap(m, tt)
	if err != nil {
		return returnT.Interface().(t), err
	}
	returnT.Set(v)
	return returnT.Interface().(t), nil
}

func fromMap(m map[string]any, tt reflect.Type) (reflect.Value, error) {
	returnT := reflect.New(tt).Elem()
	for i := range tt.NumField() {
		f := tt.Field(i)
		v := strings.Split(f.Tag.Get("json"), ",")[0] // use split to ignore tag "options" like omitempty, etc.
		if v == "" || !f.IsExported() {
			continue
		}
		omitempty := strings.Contains(f.Tag.Get("json"), "omitempty")
		if mval, ok := m[v]; ok {
			//mvalType := reflect.TypeOf(mval)
			mvalValue := reflect.ValueOf(mval)
			// Handle adding mval
			vf := returnT.FieldByName(f.Name)
			// If the struct is using a point to a field, get the underlying elem
			kind := vf.Kind()
			usePointer := false
			if vf.Kind() == reflect.Pointer {
				kind = vf.Type().Elem().Kind()
				usePointer = true
			}
			switch kind {
			// These should be immediatly assignable.
			case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32,
				reflect.Float64, reflect.String, reflect.Map:
				if mvalValue.CanConvert(vf.Type()) {
					vf.Set(mvalValue.Convert(vf.Type()))
				} else if usePointer {
					x := reflect.New(vf.Type().Elem())
					x.Elem().Set(mvalValue)
					vf.Set(x)
				} else {
					vf.Set(mvalValue)
				}
			// These need to get called recurisivelly
			case reflect.Struct:
				if mapVal, ok := mvalValue.Interface().(map[string]any); ok {
					n, err := fromMap(mapVal, vf.Type())
					if err != nil {
						return returnT, NewJsonRpcError(ParseError, m)
					}
					if usePointer {
						x := reflect.New(vf.Type().Elem())
						x.Elem().Set(mvalValue)
						vf.Set(x)
					} else {
						vf.Set(n)
					}
				} else {
					return returnT, NewJsonRpcError(ParseError, m)
				}
			case reflect.Interface:
				// Todo: we will have to use the custom Json Marshal for the the
				// interface to determine how to do this.
				return returnT, NewJsonRpcError(InternalError, m)
			// These need to get handled by the slice from
			case reflect.Array, reflect.Slice:
				if mapVal, ok := mvalValue.Interface().([]any); ok {
					n, err := fromArray(mapVal, vf.Type())
					if err != nil {
						return returnT, NewJsonRpcError(ParseError, m)
					}
					if usePointer {
						x := reflect.New(vf.Type().Elem())
						x.Elem().Set(mvalValue)
						vf.Set(x)
					} else {
						vf.Set(n)
					}
				} else {
					return returnT, NewJsonRpcError(ParseError, m)
				}
			default:
				continue
			}

		} else if !omitempty {
			return returnT, NewJsonRpcError(ParseError, m)
		}
	}
	return returnT, nil
}

func (n *TypedParameters[t]) fromArray(m []any, tt reflect.Type) (t, error) {
	returnT := reflect.New(tt).Elem()
	v, err := fromArray(m, tt)
	if err != nil {
		return returnT.Interface().(t), err
	}
	returnT.Set(v)
	return returnT.Interface().(t), nil
}

func fromArray(a []any, tt reflect.Type) (reflect.Value, error) {
	returnT := reflect.New(tt).Elem()
	sliceType := tt.Elem()
	isArray := tt.Kind() == reflect.Array
	switch sliceType.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32,
		reflect.Float64, reflect.String, reflect.Map:
		for i, v := range a {
			mvalValue := reflect.ValueOf(v)
			addValue := mvalValue
			if mvalValue.CanConvert(sliceType) {
				addValue = mvalValue.Convert(sliceType)
			}
			if isArray {
				returnT.Index(i).Set(addValue)
			} else {
				returnT = reflect.Append(returnT, addValue)
			}
		}
	case reflect.Struct:
		for i, v := range a {
			var m map[string]any
			var ok bool
			if m, ok = v.(map[string]any); !ok {
				return returnT, NewJsonRpcError(ParseError, a)
			}
			v, err := fromMap(m, sliceType)
			if err != nil {
				return returnT, err
			}

			if isArray {
				returnT.Index(i).Set(v)
			} else {
				returnT = reflect.Append(returnT, v)
			}
		}
	case reflect.Interface:
		if sliceType.NumMethod() == 0 {
			for i, v := range a {
				if isArray {
					returnT.Index(i).Set(reflect.ValueOf(v))
				} else {
					returnT = reflect.Append(returnT, reflect.ValueOf(v))
				}
			}
		} else {
			// TODO: Handle custom json for interfaces that are not empty type
			return returnT, NewJsonRpcError(InternalError, a)
		}
	case reflect.Array, reflect.Slice:
		for i, v := range a {
			cv := v.([]any)
			x, err := fromArray(cv, sliceType)
			if err != nil {
				return returnT, NewJsonRpcError(ParseError, a)
			}
			if isArray {
				returnT.Index(i).Set(x)
			} else {
				returnT = reflect.Append(returnT, x)
			}
		}
	}

	return returnT, nil
}

var _ Parameters = &TypedParameters[struct{}]{}
