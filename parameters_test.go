package jsonrpc2_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/shawn-hurley/jsonrpc2"
)

type testSimpleArgs struct {
	SOmit     string                 `json:"somit,omitempty"`
	S         string                 `json:"s,omitempty"`
	PtrString *string                `json:"ptrstr"`
	I         int                    `json:"i"`
	I8        int8                   `json:"i8"`
	I16       int16                  `json:"i16"`
	I32       int32                  `json:"i32"`
	I64       int64                  `json:"i64"`
	Ui        uint                   `json:"ui"`
	Ui8       uint8                  `json:"ui8"`
	Ui16      uint16                 `json:"ui16"`
	Ui32      uint32                 `json:"ui32"`
	Ui64      uint64                 `json:"ui64"`
	F32       float32                `json:"f32"`
	F64       float64                `json:"f64"`
	B         bool                   `json:"b"`
	M         map[string]interface{} `json:"mapTest"`
}

type testNestedStruct struct {
	SimpleArgs testSimpleArgs `json:"args"`
	News       string         `json:"News"`
}

type smallArgs struct {
	A string `json:"a"`
}

func (s *smallArgs) Test() string {
	return s.A
}

type TestInterface interface {
	Test() string
}

type testInterfaceStruct struct {
	Tester TestInterface `json:"tester"`
}

var s = "testing"
var smallArgsInterface TestInterface = &smallArgs{
	A: "testing",
}
var internalError = jsonrpc2.NewJsonRpcError(jsonrpc2.InternalError, nil)

func TestSimpleNamedParameters(t *testing.T) {
	testCases := []struct {
		parameters   *jsonrpc2.TypedParameters[testSimpleArgs]
		testMap      map[string]any
		expectedArgs testSimpleArgs
	}{
		{
			parameters: &jsonrpc2.TypedParameters[testSimpleArgs]{},
			testMap: map[string]any{
				"s":      "testString",
				"ptrstr": "testing",
				"i":      float64(12),
				"i8":     float64(12),
				"i16":    float64(12),
				"i32":    float64(12),
				"i64":    float64(12),
				"ui":     float64(12),
				"ui8":    float64(12),
				"ui16":   float64(12),
				"ui32":   float64(12),
				"ui64":   float64(12),
				"f32":    float64(12),
				"f64":    float64(12),
				"b":      true,
				"mapTest": map[string]any{
					"testing123": "laskdjf",
					"array":      []any{float64(12), float64(14)},
				},
			},
			expectedArgs: testSimpleArgs{
				S:         "testString",
				PtrString: &s,
				I:         int(12),
				I8:        int8(12),
				I16:       int16(12),
				I32:       int32(12),
				I64:       int64(12),
				Ui:        uint(12),
				Ui8:       uint8(12),
				Ui16:      uint16(12),
				Ui32:      uint32(12),
				Ui64:      uint64(12),
				F32:       float32(12),
				F64:       float64(12),
				B:         true,
				M: map[string]any{
					"testing123": "laskdjf",
					"array":      []any{float64(12), float64(14)},
				},
			},
		},
	}

	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err != nil {
			t.Log("err", err)
			t.Fatalf("error converting from testMap")
		}
		if !reflect.DeepEqual(tc.expectedArgs, tc.parameters.Args) {
			fmt.Printf("%#v\n\n%#v", tc.expectedArgs, tc.parameters.Args)
			t.Fail()
		}
	}
}

func TestNestedStruct(t *testing.T) {
	testCases := []struct {
		parameters   jsonrpc2.TypedParameters[testNestedStruct]
		testMap      map[string]any
		expectedArgs testNestedStruct
	}{
		{
			parameters: jsonrpc2.TypedParameters[testNestedStruct]{},
			testMap: map[string]any{
				"News": "testing",
				"args": map[string]any{
					"s":      "testString",
					"ptrstr": "testing",
					"i":      float64(12),
					"i8":     float64(12),
					"i16":    float64(12),
					"i32":    float64(12),
					"i64":    float64(12),
					"ui":     float64(12),
					"ui8":    float64(12),
					"ui16":   float64(12),
					"ui32":   float64(12),
					"ui64":   float64(12),
					"f32":    float64(12),
					"f64":    float64(12),
					"b":      true,
					"mapTest": map[string]any{
						"testing123": "laskdjf",
						"array":      []any{float64(12), float64(14)},
					},
				},
			},
			expectedArgs: testNestedStruct{
				News: "testing",
				SimpleArgs: testSimpleArgs{
					S:         "testString",
					PtrString: &s,
					I:         int(12),
					I8:        int8(12),
					I16:       int16(12),
					I32:       int32(12),
					I64:       int64(12),
					Ui:        uint(12),
					Ui8:       uint8(12),
					Ui16:      uint16(12),
					Ui32:      uint32(12),
					Ui64:      uint64(12),
					F32:       float32(12),
					F64:       float64(12),
					B:         true,
					M: map[string]any{
						"testing123": "laskdjf",
						"array":      []any{float64(12), float64(14)},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err != nil {
			t.Log("err", err)
			t.Fatalf("error converting from testMap")
		}
		if !reflect.DeepEqual(tc.expectedArgs, tc.parameters.Args) {
			fmt.Printf("%#v\n\n%#v", tc.expectedArgs, tc.parameters.Args)
			t.Fail()
		}
	}
}

func TestIterface(t *testing.T) {
	testCases := []struct {
		parameters   *jsonrpc2.TypedParameters[testInterfaceStruct]
		testMap      map[string]any
		expectedArgs testInterfaceStruct
		expectError  *jsonrpc2.JsonRpcError
	}{
		{
			parameters: &jsonrpc2.TypedParameters[testInterfaceStruct]{},
			testMap: map[string]any{
				"tester": map[string]any{
					"a": "testing",
				},
			},
			expectedArgs: testInterfaceStruct{
				Tester: smallArgsInterface,
			},
			expectError: internalError,
		},
	}

	// TODO: figure out how to make sure it is an internal error code
	// until we have it working more fully.
	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err == nil {
			fmt.Printf("%#v", err)
			t.Log("err", err)
			t.Fail()
		}
	}
}

func TestSliceParameter(t *testing.T) {
	testCases := []struct {
		parameters   *jsonrpc2.TypedParameters[[]smallArgs]
		testMap      []any
		expectedArgs []smallArgs
	}{
		{
			parameters: &jsonrpc2.TypedParameters[[]smallArgs]{},
			testMap: []any{
				map[string]interface{}{
					"a": "testing",
				},
			},
			expectedArgs: []smallArgs{
				{
					A: "testing",
				},
			},
		},
	}

	// TODO: figure out how to make sure it is an internal error code
	// until we have it working more fully.
	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err != nil {
			t.Log("here", err)
			t.Fatal()
		}
		if !reflect.DeepEqual(tc.expectedArgs, tc.parameters.Args) {
			fmt.Printf("%#v\n\n%#v", tc.expectedArgs, tc.parameters.Args)
			t.Fail()
		}
	}
}

func TestSliceSimple(t *testing.T) {
	testCases := []struct {
		parameters   *jsonrpc2.TypedParameters[[]string]
		testMap      []any
		expectedArgs []string
	}{
		{
			parameters:   &jsonrpc2.TypedParameters[[]string]{},
			testMap:      []any{"testing"},
			expectedArgs: []string{"testing"},
		},
	}

	// TODO: figure out how to make sure it is an internal error code
	// until we have it working more fully.
	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err != nil {
			t.Log("here", err)
			t.Fatal()
		}
		if !reflect.DeepEqual(tc.expectedArgs, tc.parameters.Args) {
			fmt.Printf("%#v\n\n%#v", tc.expectedArgs, tc.parameters.Args)
			t.Fail()
		}
	}
}

func TestSliceAny(t *testing.T) {
	testCases := []struct {
		parameters   *jsonrpc2.TypedParameters[[]any]
		testMap      []any
		expectedArgs []any
	}{
		{
			parameters:   &jsonrpc2.TypedParameters[[]any]{},
			testMap:      []any{"testing", map[string]any{"a": "testing"}},
			expectedArgs: []any{"testing", map[string]any{"a": "testing"}},
		},
	}

	// TODO: figure out how to make sure it is an internal error code
	// until we have it working more fully.
	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err != nil {
			t.Log("here", err)
			t.Fatal()
		}
		if !reflect.DeepEqual(tc.expectedArgs, tc.parameters.Args) {
			fmt.Printf("%#v\n\n%#v", tc.expectedArgs, tc.parameters.Args)
			t.Fail()
		}
	}
}

func TestSliceOfSlice(t *testing.T) {
	testCases := []struct {
		parameters   *jsonrpc2.TypedParameters[[][]string]
		testMap      []any
		expectedArgs [][]string
	}{
		{
			parameters:   &jsonrpc2.TypedParameters[[][]string]{},
			testMap:      []any{[]any{"testing"}},
			expectedArgs: [][]string{{"testing"}},
		},
	}

	// TODO: figure out how to make sure it is an internal error code
	// until we have it working more fully.
	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err != nil {
			t.Log("here", err)
			t.Fatal()
		}
		if !reflect.DeepEqual(tc.expectedArgs, tc.parameters.Args) {
			fmt.Printf("%#v\n\n%#v", tc.expectedArgs, tc.parameters.Args)
			t.Fail()
		}
	}
}

func TestArraySimple(t *testing.T) {
	testCases := []struct {
		parameters   *jsonrpc2.TypedParameters[[1]string]
		testMap      []any
		expectedArgs [1]string
	}{
		{
			parameters:   &jsonrpc2.TypedParameters[[1]string]{},
			testMap:      []any{"testing"},
			expectedArgs: [1]string{"testing"},
		},
	}

	// TODO: figure out how to make sure it is an internal error code
	// until we have it working more fully.
	for _, tc := range testCases {
		err := tc.parameters.FromAny(tc.testMap)
		if err != nil {
			t.Log("here", err)
			t.Fatal()
		}
		if !reflect.DeepEqual(tc.expectedArgs, tc.parameters.Args) {
			fmt.Printf("%#v\n\n%#v", tc.expectedArgs, tc.parameters.Args)
			t.Fail()
		}
	}
}
