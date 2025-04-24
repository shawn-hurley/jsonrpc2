package jsonrpc2_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/shawn-hurley/jsonrpc2"
)

type testListener struct {
}

// Accept an inbound connection to a server.
// It must block until an inbound connection is made, or the listener is
// shut down.
func (t *testListener) Accept(ctx context.Context) (io.ReadWriteCloser, error) {
	<-ctx.Done()
	return nil, nil
}

func (t *testListener) Close() error {
	return nil
}

type testArgs struct {
	name string
}

func NotifyFuncTest(_ context.Context, args jsonrpc2.Parameters) {
	fmt.Printf("here")
}

type testCallHandler struct {
}

func (t *testCallHandler) Handle(ctx context.Context, p jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
	return nil, nil
}

func testCallHandlerFunc(ctx context.Context, p jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
	return nil, nil
}

func TestAddingNotificationHandlers(t *testing.T) {

	testCases := []struct {
		Name                      string
		handler                   jsonrpc2.NotificationHandler
		handlerFunc               jsonrpc2.NotificationHandlerFunc
		pathRegisteredHandler     jsonrpc2.NotificationHandler
		pathRegisteredHandlerFunc jsonrpc2.NotificationHandlerFunc
		afterStartHandlerFunc     jsonrpc2.NotificationHandlerFunc
		afterStartHandler         jsonrpc2.NotificationHandler
	}{
		{
			Name:    "notification_handler",
			handler: &testNotificationHandler{},
		},
		{
			Name:        "notification_handler_func",
			handlerFunc: NotifyFuncTest,
		},
		{
			Name:                      "notification_handler_path_registered",
			handler:                   &testNotificationHandler{},
			pathRegisteredHandlerFunc: NotifyFuncTest,
		},
		{
			Name:                  "notification_handler_func_path_registered",
			handlerFunc:           NotifyFuncTest,
			pathRegisteredHandler: &testNotificationHandler{},
		},
		{
			Name:                  "notification_handler_func_after_start",
			afterStartHandlerFunc: NotifyFuncTest,
		},
		{
			Name:              "notification_handler_after_start",
			afterStartHandler: &testNotificationHandler{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s, err := jsonrpc2.NewServer(&testListener{}, jsonrpc2.ConnectionOptions{},
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(-10)})))
			if err != nil {
				t.Fatalf("got error from new server: %v", err)
			}
			if tc.handler != nil {
				err := s.HandleNotification("notify/*", tc.handler, &jsonrpc2.TypedParameters[testArgs]{})
				if err != nil {
					t.Fail()
				}
			}
			if tc.handlerFunc != nil {
				s.HandleNotificationFunc("notify/*", tc.handlerFunc, &jsonrpc2.TypedParameters[testArgs]{})
				if err != nil {
					t.Fail()
				}
			}
			if tc.pathRegisteredHandler != nil {
				err := s.HandleNotification("notify/*", tc.pathRegisteredHandler, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
			if tc.pathRegisteredHandlerFunc != nil {
				err := s.HandleNotificationFunc("notify/*", tc.pathRegisteredHandlerFunc, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}

			}
			s.Run(context.Background())
			if tc.afterStartHandler != nil {
				err := s.HandleNotification("notify/*", tc.afterStartHandler, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
			if tc.afterStartHandlerFunc != nil {
				err := s.HandleNotificationFunc("notify/*", tc.handlerFunc, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
		})
	}
}

func TestAddingCallHandlers(t *testing.T) {

	testCases := []struct {
		Name                      string
		handler                   jsonrpc2.CallHandler
		handlerFunc               jsonrpc2.CallHandlerFunc
		pathRegisteredHandler     jsonrpc2.CallHandler
		pathRegisteredHandlerFunc jsonrpc2.CallHandlerFunc
		afterStartHandlerFunc     jsonrpc2.CallHandlerFunc
		afterStartHandler         jsonrpc2.CallHandler
	}{
		{
			Name:    "call_handler",
			handler: &testCallHandler{},
		},
		{
			Name:        "call_handler_func",
			handlerFunc: testCallHandlerFunc,
		},
		{
			Name:                      "call_handler_path_registered",
			handler:                   &testCallHandler{},
			pathRegisteredHandlerFunc: testCallHandlerFunc,
		},
		{
			Name:                  "call_handler_func_path_registered",
			handlerFunc:           testCallHandlerFunc,
			pathRegisteredHandler: &testCallHandler{},
		},
		{
			Name:                  "call_handler_after_start",
			afterStartHandlerFunc: testCallHandlerFunc,
		},
		{
			Name:              "call_handler_func_after_start",
			afterStartHandler: &testCallHandler{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s, err := jsonrpc2.NewServer(&testListener{}, jsonrpc2.ConnectionOptions{},
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(-10)})))
			if err != nil {
				t.Fatalf("got error from new server: %v", err)
			}
			if tc.handler != nil {
				err := s.HandleCall("notify/*", tc.handler, &jsonrpc2.TypedParameters[testArgs]{})
				if err != nil {
					t.Fail()
				}

			}
			if tc.handlerFunc != nil {
				err := s.HandleCallFunc("notify/*", tc.handlerFunc, &jsonrpc2.TypedParameters[testArgs]{})
				if err != nil {
					t.Fail()
				}
			}
			if tc.pathRegisteredHandler != nil {
				err := s.HandleCall("notify/*", tc.pathRegisteredHandler, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
			if tc.pathRegisteredHandlerFunc != nil {
				err := s.HandleCallFunc("notify/*", tc.pathRegisteredHandlerFunc, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}

			}
			s.Run(context.Background())
			if tc.afterStartHandler != nil {
				err := s.HandleCall("notify/*", tc.afterStartHandler, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
			if tc.afterStartHandlerFunc != nil {
				err := s.HandleCallFunc("notify/*", tc.afterStartHandlerFunc, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
		})
	}
}

func TestAddingCancelHandlers(t *testing.T) {
	testCases := []struct {
		Name                      string
		handler                   jsonrpc2.CancelHandler
		handlerFunc               jsonrpc2.CancelHandlerFunc
		pathRegisteredHandler     jsonrpc2.CancelHandler
		pathRegisteredHandlerFunc jsonrpc2.CancelHandlerFunc
		afterStartHandlerFunc     jsonrpc2.CancelHandlerFunc
		afterStartHandler         jsonrpc2.CancelHandler
	}{
		{
			Name:    "call_handler",
			handler: &testCallHandler{},
		},
		{
			Name:        "call_handler_func",
			handlerFunc: testCallHandlerFunc,
		},
		{
			Name:                      "call_handler_path_registered",
			handler:                   &testCallHandler{},
			pathRegisteredHandlerFunc: testCallHandlerFunc,
		},
		{
			Name:                  "call_handler_func_path_registered",
			handlerFunc:           testCallHandlerFunc,
			pathRegisteredHandler: &testCallHandler{},
		},
		{
			Name:                  "call_handler_after_start",
			afterStartHandlerFunc: testCallHandlerFunc,
		},
		{
			Name:              "call_handler_func_after_start",
			afterStartHandler: &testCallHandler{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s, err := jsonrpc2.NewServer(&testListener{}, jsonrpc2.ConnectionOptions{},
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(-10)})))
			if err != nil {
				t.Fatalf("got error from new server: %v", err)
			}
			if tc.handler != nil {
				err := s.HandleCancel("notify/*", tc.handler, func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[testArgs]{})
				if err != nil {
					t.Fail()
				}

			}
			if tc.handlerFunc != nil {
				err := s.HandleCancelFunc("notify/*", tc.handlerFunc, func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[testArgs]{})
				if err != nil {
					t.Fail()
				}
			}
			if tc.pathRegisteredHandler != nil {
				err := s.HandleCancel("notify/*", tc.pathRegisteredHandler, func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
			if tc.pathRegisteredHandlerFunc != nil {
				err := s.HandleCancelFunc("notify/*", tc.pathRegisteredHandlerFunc, func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}

			}
			s.Run(context.Background())
			if tc.afterStartHandler != nil {
				err := s.HandleCancel("notify/*", tc.afterStartHandler, func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
			if tc.afterStartHandlerFunc != nil {
				err := s.HandleCancelFunc("notify/*", tc.afterStartHandlerFunc, func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[testArgs]{})
				if err == nil {
					t.Fail()
				}
			}
		})
	}
}
