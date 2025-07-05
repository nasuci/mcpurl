package transport

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"

	"github.com/cherrydra/mcpurl/parser"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	ErrNoTransport = errors.New("no transport specified")
)

func Transport(parsed parser.Parser) (mcp.Transport, error) {
	transportArgs := parsed.TransportArgs()
	if len(transportArgs) == 0 {
		return nil, ErrNoTransport
	}
	transportURL, err := url.Parse(transportArgs[0])
	if err != nil {
		return nil, fmt.Errorf("parse transport url: %w", err)
	}
	switch transportURL.Scheme {
	case "", "stdio":
		cmd := cmp.Or(transportURL.Host, transportURL.Path)
		command := exec.Command(cmd, transportArgs[1:]...)
		if !parsed.Silent {
			command.Stderr = os.Stderr
		}
		return mcp.NewCommandTransport(command), nil
	case "http", "https":
		return mcp.NewStreamableClientTransport(transportURL.String(), &mcp.StreamableClientTransportOptions{
			HTTPClient: &http.Client{Transport: &mcpurlRoundTripper{headers: parsed.Headers}},
		}), nil
	default:
		return nil, fmt.Errorf("unsupportd transport url scheme: %s", transportURL.Scheme)
	}
}
