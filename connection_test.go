package jsonrpc2_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/shawn-hurley/jsonrpc2"
)

type fakeConn struct {
	writeCalls chan int
}

func (f *fakeConn) Close() error {
	return nil
}
func (f *fakeConn) Read(p []byte) (n int, err error) {
	return 0, nil
}
func (f *fakeConn) Write(p []byte) (n int, err error) {
	f.writeCalls <- 1
	return 0, nil
}

type testFramer struct {
	request, response, cancelRequest jsonrpc2.MessageMap
	readerCalls                      int
	writeCalls                       chan int
	testClient                       bool
}

func (t *testFramer) Reader(r io.Reader) jsonrpc2.Reader {
	return t
}

func (t *testFramer) Writer(w io.Writer) jsonrpc2.Writer {
	return t
}

func (t *testFramer) Write(ctx context.Context, response jsonrpc2.MessageMap) (int64, error) {
	if t.testClient {
		if r, ok := response.(*jsonrpc2.Request); ok {
			if r.Method == "cancel" {
				t.cancelRequest = r
				t.writeCalls <- 1
				return 20, nil
			}
		}
		t.request = response
		t.writeCalls <- 1
	} else {
		t.response = response
		t.writeCalls <- 1
	}
	return 20, nil
}

func (t *testFramer) Read(ctx context.Context) (jsonrpc2.MessageMap, int64, error) {
	if t.testClient {
		if t.response == nil {
			return nil, 0, nil
		}
		if t.request != nil && t.readerCalls == 0 {
			t.readerCalls += 1
			return t.response, 20, nil
		}
		time.Sleep(1 * time.Second)
		return nil, 0, nil
	}
	if t.readerCalls >= 1 {
		if t.cancelRequest != nil && t.readerCalls == 1 {
			time.Sleep(2 * time.Second)
			t.readerCalls += 1
			return t.cancelRequest, 20, nil
		}
		return nil, 0, io.EOF
	}
	t.readerCalls += 1
	return t.request, 20, nil
}

type testNotificationHandler struct {
	expectedNotifications int
	gotNotificationCall   chan int
	t                     *testing.T
	requests              int
}

func (t *testNotificationHandler) Notify(ctx context.Context, params jsonrpc2.Parameters) {
	args, err := jsonrpc2.GetArgs[smallArgs](params)
	if err != nil {
		t.t.Log("unable to cast to concrete value")
		t.t.Fail()
	}
	if !reflect.DeepEqual(*args, smallArgs{A: "test"}) {
		t.t.Log("arguments are not equal", "expected", smallArgs{A: "test"}, "got", args)
		t.t.Fail()
	}
	t.requests += 1
	t.gotNotificationCall <- 1
	if t.expectedNotifications == t.requests {
		close(t.gotNotificationCall)
	}
}

type testResponse struct {
	r string
}

type testCancelParams struct {
	ID string `json:"id"`
}

var id1 string = "1"
var id2 string = "2"
var id3 string = "3"

func TestServerConnection(t *testing.T) {
	type testCase struct {
		Name          string
		writeCalls    chan int
		notifyCalls   chan int
		request       jsonrpc2.MessageMap
		cancelRequest jsonrpc2.MessageMap
		// expectedResponse for call handler
		expectedResponse jsonrpc2.MessageMap
		// expectedNotification Request
		notifyCall         int
		expectedWriteCalls int
		addToRouter        func(jsonrpc2.Router, testCase, *testing.T) error
	}
	testCases := []testCase{
		{
			Name:       "basic_call",
			writeCalls: make(chan int),
			request: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "test",
				Params: map[string]any{
					"a": "test",
				},
				ID: &id1,
			},
			expectedResponse: &jsonrpc2.Response{
				Version: "2.0",
				Result:  testResponse{r: "here"},
				Error:   nil,
				ID:      &id1,
			},
			expectedWriteCalls: 1,
			addToRouter: func(r jsonrpc2.Router, tc testCase, t *testing.T) error {
				return r.HandleCallFunc("test", func(ctx context.Context, p jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
					typedParams, ok := p.(*jsonrpc2.TypedParameters[smallArgs])
					if !ok {
						t.Log("unable to cast to concrete value")
						t.Fail()
					}
					if !reflect.DeepEqual(typedParams.Args, smallArgs{A: "test"}) {
						t.Log("arguments are not equal")
						t.Fail()
					}
					return testResponse{r: "here"}, nil
				}, &jsonrpc2.TypedParameters[smallArgs]{})
			},
		},
		{
			Name: "basic_notification",
			request: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "notification",
				Params:  map[string]any{"a": "test"},
				ID:      nil,
			},
			notifyCalls:        make(chan int),
			expectedResponse:   nil,
			expectedWriteCalls: 0,
			notifyCall:         1,
			addToRouter: func(router jsonrpc2.Router, tc testCase, t *testing.T) error {
				return router.HandleNotification("notification", &testNotificationHandler{
					expectedNotifications: tc.notifyCall,
					gotNotificationCall:   tc.notifyCalls,
					t:                     t,
					requests:              0,
				}, &jsonrpc2.TypedParameters[smallArgs]{})
			},
		},
		{
			//Batch Request
			Name:       "batch_call",
			writeCalls: make(chan int),
			request: jsonrpc2.BatchRequest{
				{
					Version: "2.0",
					Method:  "test",
					Params: map[string]any{
						"a": "test",
					},
					ID: &id1,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params: map[string]any{
						"a": "test",
					},
					ID: &id2,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params: map[string]any{
						"a": "test",
					},
					ID: &id3,
				},
			},
			expectedResponse: jsonrpc2.BatchResponse{
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					Error:   nil,
					ID:      &id1,
				},
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					Error:   nil,
					ID:      &id2,
				},
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					Error:   nil,
					ID:      &id3,
				},
			},
			expectedWriteCalls: 1,
			addToRouter: func(r jsonrpc2.Router, tc testCase, t *testing.T) error {
				return r.HandleCallFunc("test", func(ctx context.Context, p jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
					typedParams, ok := p.(*jsonrpc2.TypedParameters[smallArgs])
					if !ok {
						t.Log("unable to cast to concrete value")
						t.Fail()
					}
					if !reflect.DeepEqual(typedParams.Args, smallArgs{A: "test"}) {
						t.Log("arguments are not equal")
						t.Fail()
					}
					return testResponse{r: "here"}, nil
				}, &jsonrpc2.TypedParameters[smallArgs]{})
			},
		},
		{
			Name: "cancel_call",
			request: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "test",
				Params: map[string]any{
					"a": "test",
				},
				ID: &id1,
			},
			cancelRequest: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "cancel",
				Params:  map[string]any{"id": "1"},
			},
			writeCalls: make(chan int),
			addToRouter: func(r jsonrpc2.Router, tc testCase, t *testing.T) error {
				err := r.HandleCallFunc("test", func(ctx context.Context, p jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
					typedParams, ok := p.(*jsonrpc2.TypedParameters[smallArgs])
					if !ok {
						t.Log("unable to cast to concrete value")
						t.Fail()
					}
					if !reflect.DeepEqual(typedParams.Args, smallArgs{A: "test"}) {
						t.Log("arguments are not equal")
						t.Fail()
					}
					select {
					case <-time.After(10 * time.Second):
						return testResponse{r: "here"}, nil
					case <-ctx.Done():
						return nil, nil
					}
				}, &jsonrpc2.TypedParameters[smallArgs]{})
				if err != nil {
					return err
				}
				return r.HandleCancelFunc("cancel", func(ctx context.Context, p jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
					return nil, nil
				}, func(params jsonrpc2.Parameters) (string, error) {
					typedParams, ok := params.(*jsonrpc2.TypedParameters[testCancelParams])
					if !ok {
						t.Log("unable to cast to concrete value")
						t.Fail()
						return "", fmt.Errorf("not found")
					}
					return typedParams.Args.ID, nil
				}, &jsonrpc2.TypedParameters[testCancelParams]{})
			},
		},
		{
			//Batch Request
			Name:       "notification_in_batch_call_err",
			writeCalls: make(chan int),
			request: jsonrpc2.BatchRequest{
				{
					Version: "2.0",
					Method:  "test",
					Params: map[string]any{
						"a": "test",
					},
					ID: &id1,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params: map[string]any{
						"a": "test",
					},
					ID: &id2,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params: map[string]any{
						"a": "test",
					},
				},
			},
			expectedResponse:   jsonrpc2.NewJsonRpcError(jsonrpc2.InvalidRequest, nil),
			expectedWriteCalls: 1,
			addToRouter: func(r jsonrpc2.Router, tc testCase, t *testing.T) error {
				return r.HandleCallFunc("test", func(ctx context.Context, p jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
					typedParams, ok := p.(*jsonrpc2.TypedParameters[smallArgs])
					if !ok {
						t.Log("unable to cast to concrete value")
						t.Fail()
					}
					if !reflect.DeepEqual(typedParams.Args, smallArgs{A: "test"}) {
						t.Log("arguments are not equal")
						t.Fail()
					}
					return testResponse{r: "here"}, nil
				}, &jsonrpc2.TypedParameters[smallArgs]{})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			options := jsonrpc2.ConnectionOptions{
				Framer: &testFramer{
					request:       tc.request,
					cancelRequest: tc.cancelRequest,
					writeCalls:    tc.writeCalls,
				},
				Preempter: nil,
				Router:    jsonrpc2.NewRouter(),
			}
			tc.addToRouter(options.Router, tc, t)

			ctx, cancelFunc := context.WithCancel(context.Background())
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.Level(-10),
			}))
			_, err := jsonrpc2.NewConnection(ctx, &fakeConn{writeCalls: tc.writeCalls}, options, logger)
			if err != nil {
				t.Fatalf("unable to create new connection")
			}

			if tc.expectedWriteCalls != 0 {
				writeCalls := 0
				select {
				case <-time.After(10 * time.Second):
					t.Log("timeout waiting for write calls")
					t.Fail()
				case <-tc.writeCalls:
					t.Log("got write call")
					writeCalls += 1
					if writeCalls == tc.expectedWriteCalls {
						close(tc.writeCalls)
					}
				}
			}

			if tc.notifyCall != 0 {
				notifyCalls := 0
				select {
				case <-time.After(2 * time.Second):
					t.Log("timeout waiting for write calls")
					t.Fail()
				case <-tc.notifyCalls:
					t.Log("got notification call")
					notifyCalls += 1
				}

			}

			if tc.cancelRequest != nil {
				select {
				case <-time.After(12 * time.Second):
					// Verify no respons and no write calls
					f := options.Framer.(*testFramer)
					if f.response != nil {
						t.Log("expected no response", "res", f.response)
						t.Fail()
					}
				case <-tc.writeCalls:
					t.Log("expected no write calls")
					t.Fail()
				}
			}

			cancelFunc()
			if tc.expectedResponse != nil {
				f := options.Framer.(*testFramer)
				switch expectedResponse := tc.expectedResponse.(type) {
				case jsonrpc2.BatchResponse:
					if gotBatchResponse, ok := f.response.(jsonrpc2.BatchResponse); ok {
						for _, er := range expectedResponse {
							for _, gr := range gotBatchResponse {
								if er.ID == gr.ID {
									if !reflect.DeepEqual(er, gr) {
										t.Log("batch expected response", er, "got response", gr)
										t.Fail()
									}
								}
							}
						}
					} else {
						t.Log("did not get batch response", "response", f.response)
						t.Fail()
					}
				case *jsonrpc2.Response:
					if !reflect.DeepEqual(f.response, expectedResponse) {
						t.Log("expected response", expectedResponse, "got response", f.response)
						t.Fail()
					}
				}
			}
		})
	}

}

func TestClientConnection(t *testing.T) {
	type testCase struct {
		Name               string
		response           jsonrpc2.MessageMap
		expoectedResponse  any
		clientMethod       func(context.Context, jsonrpc2.Client, testCase) (any, error)
		method             string
		readerCalls        int
		writeCalls         chan int
		expectedWriteCalls int
		expectedRequest    jsonrpc2.MessageMap
	}

	testCases := []testCase{
		{
			Name: "test_simple_data_call",
			response: &jsonrpc2.Response{
				Version: "2.0",
				Result:  testResponse{r: "here"},
				ID:      &id1,
			},
			expoectedResponse: testResponse{r: "here"},
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				return c.Call(ctx, tc.method, smallArgs{A: "a"})
			},
			method:             "test",
			readerCalls:        1,
			expectedWriteCalls: 1,
			writeCalls:         make(chan int),
			expectedRequest: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "test",
				Params:  smallArgs{A: "a"},
				ID:      &id1,
			},
		},
		{
			Name: "test_simple_request_call",
			response: &jsonrpc2.Response{
				Version: "2.0",
				Result:  testResponse{r: "here"},
				ID:      &id1,
			},
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				return c.Call(ctx, tc.method, jsonrpc2.Request{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
					ID:      &id1,
				})
			},
			expoectedResponse:  testResponse{r: "here"},
			method:             "test",
			readerCalls:        1,
			expectedWriteCalls: 1,
			writeCalls:         make(chan int),
			expectedRequest: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "test",
				Params:  smallArgs{A: "a"},
				ID:      &id1,
			},
		},
		{
			Name: "test_simple_request_notify",
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				err := c.Notify(ctx, tc.method, &jsonrpc2.Request{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
				})
				return nil, err
			},
			method:             "test",
			expectedWriteCalls: 1,
			writeCalls:         make(chan int),
			expectedRequest: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "test",
				Params:  smallArgs{A: "a"},
			},
		},
		{
			Name: "test_simple_data_notify",
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				err := c.Notify(ctx, tc.method, smallArgs{A: "a"})
				return nil, err
			},
			method:             "test",
			expectedWriteCalls: 1,
			writeCalls:         make(chan int),
			expectedRequest: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "test",
				Params:  smallArgs{A: "a"},
			},
		},
		{
			Name: "test_batch_notify",
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				err := c.Notify(ctx, tc.method, jsonrpc2.BatchRequest{
					{
						Version: "2.0",
						Method:  "notify1",
						Params:  smallArgs{A: "a"},
					},
					{
						Version: "2.0",
						Method:  "notify2",
						Params:  smallArgs{A: "a"},
					},
					{
						Version: "2.0",
						Method:  "notify3",
						Params:  smallArgs{A: "a"},
					},
				})
				return nil, err
			},
			method:             "",
			expectedWriteCalls: 1,
			writeCalls:         make(chan int),
			expectedRequest: jsonrpc2.BatchRequest{
				{
					Version: "2.0",
					Method:  "notify1",
					Params:  smallArgs{A: "a"},
				},
				{
					Version: "2.0",
					Method:  "notify2",
					Params:  smallArgs{A: "a"},
				},
				{
					Version: "2.0",
					Method:  "notify3",
					Params:  smallArgs{A: "a"},
				},
			},
		},
		{
			Name: "test_batch_call",
			response: jsonrpc2.BatchResponse{
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id1,
				},
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id2,
				},
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id3,
				},
			},
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				return c.Call(ctx, tc.method, jsonrpc2.BatchRequest{
					{
						Version: "2.0",
						Method:  "test",
						Params:  smallArgs{A: "a"},
						ID:      &id1,
					},
					{
						Version: "2.0",
						Method:  "test",
						Params:  smallArgs{A: "a"},
						ID:      &id2,
					},
					{
						Version: "2.0",
						Method:  "test",
						Params:  smallArgs{A: "a"},
						ID:      &id3,
					},
				})
			},
			expoectedResponse: []any{
				&jsonrpc2.Response{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id1,
				},
				&jsonrpc2.Response{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id2,
				},
				&jsonrpc2.Response{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id3,
				},
			},
			method:             "test",
			readerCalls:        1,
			expectedWriteCalls: 1,
			writeCalls:         make(chan int),
			expectedRequest: jsonrpc2.BatchRequest{
				{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
					ID:      &id1,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
					ID:      &id2,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
					ID:      &id3,
				},
			},
		},
		{
			Name: "test_batch_call_with_notification",
			response: jsonrpc2.BatchResponse{
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id1,
				},
				{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id2,
				},
			},
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				return c.Call(ctx, tc.method, jsonrpc2.BatchRequest{
					{
						Version: "2.0",
						Method:  "test",
						Params:  smallArgs{A: "a"},
						ID:      &id1,
					},
					{
						Version: "2.0",
						Method:  "test",
						Params:  smallArgs{A: "a"},
						ID:      &id2,
					},
					{
						Version: "2.0",
						Method:  "test",
						Params:  smallArgs{A: "a"},
					},
				})
			},
			expoectedResponse: []any{
				&jsonrpc2.Response{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id1,
				},
				&jsonrpc2.Response{
					Version: "2.0",
					Result:  testResponse{r: "here"},
					ID:      &id2,
				},
			},
			method:             "test",
			readerCalls:        1,
			expectedWriteCalls: 1,
			writeCalls:         make(chan int),
			expectedRequest: jsonrpc2.BatchRequest{
				{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
					ID:      &id1,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
					ID:      &id2,
				},
				{
					Version: "2.0",
					Method:  "test",
					Params:  smallArgs{A: "a"},
				},
			},
		},
		{
			Name: "test_simple_cancel",
			clientMethod: func(ctx context.Context, c jsonrpc2.Client, tc testCase) (any, error) {
				ch, id, err := c.CallAsync(ctx, tc.method, smallArgs{A: "a"})
				if err != nil {
					return nil, err
				}
				time.Sleep(1 * time.Second)
				err = c.Cancel(ctx, "cancel", smallArgs{A: "a"}, *id)
				if err != nil {
					return nil, err
				}
				_, ok := <-ch
				if ok {
					return nil, fmt.Errorf("should get closed return channel")
				}
				return nil, nil
			},
			method:             "test",
			readerCalls:        0,
			expectedWriteCalls: 2,
			writeCalls:         make(chan int),
			expectedRequest: &jsonrpc2.Request{
				Version: "2.0",
				Method:  "test",
				Params:  smallArgs{A: "a"},
				ID:      &id1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			framer := &testFramer{
				writeCalls: tc.writeCalls,
				testClient: true,
			}

			options := jsonrpc2.ConnectionOptions{
				Framer: framer,
			}
			ctx, cancel := context.WithCancel(t.Context())
			log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(-10)}))
			conn, err := jsonrpc2.NewConnection(ctx, &fakeConn{}, options, log)
			if err != nil {
				t.Fatalf("could not create connection-err: %v", err)
			}
			gotWriteCalls := 0
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-tc.writeCalls:
						gotWriteCalls += 1
					}
				}
			}()

			client, err := jsonrpc2.NewDefaultClient(conn, log)
			if err != nil {
				t.Fatalf("could not create client-err: %v", err)
			}
			gotResp, err := tc.clientMethod(ctx, client, tc)

			cancel()
			if err != nil {
				t.Fatalf("unable to write request")
			}
			eresp, batchResponseOk := tc.expoectedResponse.([]any)
			if resp, ok := gotResp.([]any); ok && batchResponseOk {
				if len(resp) != len(eresp) {
					t.Log("response length are not equal", "got", len(resp), "expected", len(eresp))
					t.Fail()
					return
				}
				for _, r := range resp {
					found := false
					rr := r.(*jsonrpc2.Response)
					for _, er := range eresp {
						rer := er.(*jsonrpc2.Response)
						if rer.ID == rr.ID {
							found = true
							if !reflect.DeepEqual(rr, rer) {
								t.Log("did not get the expected reader calls", "got", rr, "expected", rer)
								t.Fail()
							}
							break
						}
					}
					if !found {
						t.Log("got unexpected response", "got", r)
					}
				}
			} else {
				if !reflect.DeepEqual(gotResp, tc.expoectedResponse) {
					t.Log("did not get the expected response", "gotResponse", fmt.Sprintf("%#v", gotResp), "expected", fmt.Sprintf("%#v", tc.expoectedResponse))
					t.Fail()
				}
			}
			if !reflect.DeepEqual(framer.request, tc.expectedRequest) {
				t.Log("did not get the expected request", "request", framer.request, "expected", tc.expectedRequest)
				t.Fail()
			}
			if gotWriteCalls != tc.expectedWriteCalls {
				t.Log("did not get the expected write calls", "got", gotWriteCalls, "expected", tc.expectedWriteCalls)
				t.Fail()
			}
			if framer.readerCalls != tc.readerCalls {
				t.Log("did not get the expected reader calls", "got", gotWriteCalls, "expected", tc.expectedWriteCalls)
				t.Fail()
			}
		})

	}

}
