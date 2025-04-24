package jsonrpc2

import (
	"errors"
	"fmt"
	"strings"
)

type cancelHandler struct {
	handler     CancelHandler
	getCancelID GetCancelID
	Parameters  Parameters
}

type notificationHandler struct {
	p Parameters
	h NotificationHandler
}

type callHandler struct {
	p Parameters
	h CallHandler
}

type Router interface {
	RouteCall(string) (*callHandler, error)
	RouteNotifaction(string) (*notificationHandler, error)
	IsCancelRequest(string) (*cancelHandler, bool)
	HandleCall(string, CallHandler, Parameters) error
	HandleCallFunc(string, CallHandlerFunc, Parameters) error
	HandleNotification(string, NotificationHandler, Parameters) error
	HandleNotificationFunc(string, NotificationHandlerFunc, Parameters) error
	HandleCancel(string, CancelHandler, GetCancelID, Parameters) error
	HandleCancelFunc(string, CancelHandlerFunc, GetCancelID, Parameters) error
	IsRunning()
}

func NewRouter() Router {
	return &defaultRouter{
		callHandlers:         map[string]*callHandler{},
		notificationHandlers: map[string]*notificationHandler{},
		cancelHandler:        map[string]*cancelHandler{},
	}
}

type defaultRouter struct {
	callHandlers         map[string]*callHandler
	notificationHandlers map[string]*notificationHandler
	cancelHandler        map[string]*cancelHandler
	running              bool
}

func (d *defaultRouter) IsRunning() {
	d.running = true
}

func findPartialPath(wantedPath, registerPath string) bool {
	// if the path is star will always work
	if registerPath == "*" {
		return true
	}
	// Found a partial path
	if !strings.Contains(registerPath, "*") {
		return false
	}
	indexOfStar := strings.Index(registerPath, "*")

	return strings.HasPrefix(wantedPath, registerPath[:indexOfStar])

}
func (d *defaultRouter) RouteCall(path string) (*callHandler, error) {
	if h, ok := d.callHandlers[path]; ok {
		return h, nil
	}
	// If we didn't find the path then we need to search for any partial paths
	for p, handler := range d.callHandlers {
		if findPartialPath(path, p) {
			return handler, nil
		}
	}
	return nil, NewJsonRpcError(MethodNotFound, nil)
}

func (d *defaultRouter) RouteNotifaction(path string) (*notificationHandler, error) {
	if h, ok := d.notificationHandlers[path]; ok {
		return h, nil
	}
	for p, handler := range d.notificationHandlers {
		if findPartialPath(path, p) {
			return handler, nil
		}
	}
	return nil, NewJsonRpcError(MethodNotFound, nil)
}

func (d *defaultRouter) IsCancelRequest(path string) (*cancelHandler, bool) {
	if h, ok := d.cancelHandler[path]; ok {
		return h, true
	}
	for p, handler := range d.cancelHandler {
		if findPartialPath(path, p) {
			return handler, true
		}
	}
	return nil, false
}

func verifyPartialPath(path string) error {
	if strings.Contains(path, "*") {
		if split := strings.Split(path, "*"); len(split) != 2 {
			return fmt.Errorf("can only handle partial paths of * at the end of a path")
		} else if split[len(split)-1] != "" {
			return fmt.Errorf("can only handle partial paths of * at the end of a path")
		}
	}
	return nil
}

func (d *defaultRouter) HandleCall(path string, handler CallHandler, params Parameters) error {
	if d.running {
		return errors.New("can not add handlers after server has started")
	}
	err := verifyPartialPath(path)
	if err != nil {
		return err
	}
	if _, ok := d.callHandlers[path]; ok {
		return fmt.Errorf("path: %s is already registered", path)
	}
	d.callHandlers[path] = &callHandler{h: handler, p: params}
	return nil
}

// HandleFunc will map either partial methods
// notifications/* or notifications.* to a given handler or a fully defined
// path notifications/initialized
func (d *defaultRouter) HandleCallFunc(path string, handlerFunc CallHandlerFunc, params Parameters) error {
	if d.running {
		return errors.New("can not add handlers after server has started")
	}
	err := verifyPartialPath(path)
	if err != nil {
		return err
	}
	if _, ok := d.callHandlers[path]; ok {
		return fmt.Errorf("path: %s is already registered", path)
	}
	d.callHandlers[path] = &callHandler{h: &callHandlerFuncWrapper{handlerFunc: handlerFunc}, p: params}
	return nil
}

// Handle will map either partial methods
// notifications/* or notifications.* to a given handler or a fully defined
// path notifications/initialized
func (d *defaultRouter) HandleNotification(path string, handler NotificationHandler, parameters Parameters) error {
	if d.running {
		return errors.New("can not add handlers after server has started")
	}
	err := verifyPartialPath(path)
	if err != nil {
		return err
	}
	if _, ok := d.notificationHandlers[path]; ok {
		return fmt.Errorf("path: %s is already registered", path)
	}
	d.notificationHandlers[path] = &notificationHandler{p: parameters, h: handler}
	return nil
}

// HandleFunc will map either partial methods
// notifications/* or notifications.* to a given handler or a fully defined
// path notifications/initialized
func (d *defaultRouter) HandleNotificationFunc(path string, handlerFunc NotificationHandlerFunc, params Parameters) error {
	if d.running {
		return errors.New("can not add handlers after server has started")
	}
	err := verifyPartialPath(path)
	if err != nil {
		return err
	}
	if _, ok := d.notificationHandlers[path]; ok {
		return fmt.Errorf("path: %s is already registered", path)
	}
	d.notificationHandlers[path] = &notificationHandler{h: &notficationHandlerFuncWrapper{handlerFunc: handlerFunc}, p: params}
	return nil
}

func (d *defaultRouter) HandleCancel(path string, handler CancelHandler, getCancelID GetCancelID, params Parameters) error {
	if d.running {
		return errors.New("can not add handlers after server has started")
	}
	err := verifyPartialPath(path)
	if err != nil {
		return err
	}
	if _, ok := d.cancelHandler[path]; ok {
		return fmt.Errorf("path: %s is already registered", path)
	}
	d.cancelHandler[path] = &cancelHandler{
		handler:     handler,
		getCancelID: getCancelID,
	}
	return nil
}

// HandleFunc will map either partial methods
// notifications/* or notifications.* to a given handler or a fully defined
// path notifications/initialized
func (d *defaultRouter) HandleCancelFunc(path string, handlerFunc CancelHandlerFunc, getCancelID GetCancelID, params Parameters) error {
	if d.running {
		return errors.New("can not add handlers after server has started")
	}
	err := verifyPartialPath(path)
	if err != nil {
		return err
	}
	if _, ok := d.cancelHandler[path]; ok {
		return fmt.Errorf("path: %s is already registered", path)
	}
	d.cancelHandler[path] = &cancelHandler{
		handler:     &cancelHandlerFuncWrapper{handlerFunc: handlerFunc},
		getCancelID: getCancelID,
		Parameters:  params,
	}
	return nil
}
