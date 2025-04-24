package jsonrpc2

import (
	"context"
	"io"
	"log/slog"
)

// This will define where connections will come from.
type Listener interface {
	// Accept an inbound connection to a server.
	// It must block until an inbound connection is made, or the listener is
	// shut down.
	Accept(context.Context) (io.ReadWriteCloser, error)

	// Close is used to ask a listener to stop accepting new connections.
	Close() error
}

// The Server's only responsilbilty is to listen for connections
// and start the processing of the connection in a go routine.
// each connection will have access to the handlers that are defined
// on the server
type Server struct {
	Router

	// handlers will map either partial methods
	// notifications/* or notifications.* to a given handler or a fully defined
	// path notifications/initialized
	listen Listener

	options ConnectionOptions
	running bool
	log     *slog.Logger

	Err error
}

func NewServer(listener Listener, options ConnectionOptions, log *slog.Logger) (*Server, error) {
	if options.Router == nil {
		options.Router = NewRouter()
	}
	return &Server{
		listen:  listener,
		options: options,
		log:     log,
		Router:  options.Router,
	}, nil
}

// Handle will map either partial methods
// notifications/* or notifications.* to a given handler or a fully defined
// path notifications/initialized

// Run will not block, but will start listening for new connections on the listener
// to wait for the server use Wait, or to stop gracefully use Shutdown.
// you can also use the context to stop the server gracefully.
func (s *Server) Run(ctx context.Context) error {
	s.Router.IsRunning()
	go s.run(ctx)
	if s.Err != nil {
		return s.Err
	}
	return nil
}

func (s *Server) run(ctx context.Context) {
	s.running = true
	for {
		conn, err := s.listen.Accept(ctx)
		if err != nil {
			s.Err = err
			return
		}
		s.log.Debug("recieved new connection", "connection", conn)

		// Connection specific context
		ctx := context.WithoutCancel(ctx)

		// TODO: keep track of active connections
		_, err = NewConnection(ctx, conn, s.options, s.log)
		if err != nil {
			s.Err = err
			return
		}
	}
}

// TODO:

// TODO: shawn-hurley determine if we want to have a binder
// // Binder builds a connection configuration.
// // This may be used in servers to generate a new configuration per connection.
// // ConnectionOptions itself implements Binder returning itself unmodified, to
// // allow for the simple cases where no per connection information is needed.
// type Binder interface {
// 	// Bind is invoked when creating a new connection.
// 	// The connection is not ready to use when Bind is called.
// 	Bind(context.Context, *Connection) (ConnectionOptions, error)
// }
