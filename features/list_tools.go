package features

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s ServerFeatures) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	if s.Session == nil {
		return nil, ErrNoSession
	}
	params := &mcp.ListToolsParams{}
	var tools []*mcp.Tool
	for {
		result, err := s.Session.ListTools(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list tools: %w", err)
		}
		tools = append(tools, result.Tools...)
		if result.NextCursor == "" {
			break
		}
		params.Cursor = result.NextCursor
	}
	return tools, nil
}

func (s *ServerFeatures) PrintTools(ctx context.Context) error {
	tools, err := s.ListTools(ctx)
	if err != nil {
		return err
	}
	for _, tool := range tools {
		json.NewEncoder(cmp.Or(s.Out, os.Stdout)).Encode(tool)
	}
	return nil
}
