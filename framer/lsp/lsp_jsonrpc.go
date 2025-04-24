// lsp package is responsible for framing the dialect of jsonrpc for LSP.
// for more information: https://microsoft.github.io/language-server-protocol/specifications/base/0.9/specification/
package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/shawn-hurley/jsonrpc2"
)

const headerString = "Content-Length: %d\r\n\r\n"

type Framer struct {
	Log *slog.Logger
}

// Reader implements jsonrpc2.Framer.
func (f *Framer) Reader(r io.Reader) jsonrpc2.Reader {
	scanner := bufio.NewScanner(r)
	l := &lspReader{
		scanner: scanner,
		log:     f.Log.With("name", "lsp-jsonrpc-reader"),
	}
	l.scanner.Split(l.splitLanguageServer)
	return l

}

// Writer implements jsonrpc2.Framer.
func (f *Framer) Writer(w io.Writer) jsonrpc2.Writer {
	return &lspWriter{
		writer: w,
		log:    f.Log.With("name", "lsp-jsonrpc-writer"),
	}
}

var _ jsonrpc2.Framer = &Framer{}

type lspReader struct {
	scanner *bufio.Scanner
	log     *slog.Logger
}

// Reader implements jsonrpc2.Framer.
func (l *lspReader) Read(ctx context.Context) (jsonrpc2.MessageMap, int64, error) {
	for !l.scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, 0, io.EOF
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
	l.log.Log(ctx, slog.Level(-7), "reading request header", "body", l.scanner.Text())
	m, err := jsonrpc2.GetMessage(l.scanner.Bytes())
	return m, int64(len(l.scanner.Bytes())), err
}

type lspWriter struct {
	writer io.Writer
	log    *slog.Logger
}

// Writer implements jsonrpc2.Framer.
func (l *lspWriter) Write(ctx context.Context, message jsonrpc2.MessageMap) (int64, error) {
	l.log.Log(ctx, slog.Level(-7), "write request")
	b, err := json.Marshal(message)
	if err != nil {
		l.log.Error("unable to marshal message", "err", err)
		return 0, err
	}
	headerBytes, err := l.writer.Write(([]byte(fmt.Sprintf(headerString, len(b)))))
	if err != nil {
		l.log.Error("unable to write header string", "err", err)
		return 0, err
	}
	bodyBytes, err := l.writer.Write(b)
	if err != nil {
		l.log.Error("unable to write request body", "err", err)
		return int64(headerBytes), err
	}
	l.log.Log(ctx, slog.Level(-7), "finished writing request", "err", err, "wrote", headerBytes+bodyBytes)
	return int64(headerBytes) + int64(bodyBytes), nil
}

func (l *lspReader) splitLanguageServer(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// Find any headers
	if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
		readContentHeader := data[0:i]
		l.log.Log(context.TODO(), slog.Level(-5), "found header", "header", fmt.Sprintf("%q", readContentHeader))
		if !strings.Contains(string(readContentHeader), "Content-Length") {
			return 0, nil, fmt.Errorf("found header separator but not content-length header")
		}
		pieces := strings.Split(string(readContentHeader), ":")
		if len(pieces) != 2 {
			return 0, nil, fmt.Errorf("invalid pieces")
		}
		addedLength, err := strconv.Atoi(strings.TrimSpace(pieces[1]))
		if err != nil {
			return 0, nil, err
		}
		if i+4+addedLength > len(data) {
			// wait for the buffer to fill up
			l.log.Log(context.TODO(), slog.Level(-5), "waiting for more data from buffer in scanner")
			return 0, nil, nil
		}
		return i + addedLength + 4, data[i+4 : i+4+addedLength], nil
	}
	if atEOF {
		l.log.Log(context.TODO(), slog.Level(-5), "scanner at EOF")
		return len(data), data, nil
	}
	return 0, nil, nil
}
