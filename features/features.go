package features

import (
	"errors"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	ErrNoSession = errors.New("no session")
)

type ServerFeatures struct {
	Session *mcp.ClientSession
	Out     *os.File
}
