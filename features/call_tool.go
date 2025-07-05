package features

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *ServerFeatures) CallTool(ctx context.Context, tool, data string) error {
	params := map[string]any{}
	if data != "" {
		if err := json.Unmarshal([]byte(data), &params); err != nil {
			return fmt.Errorf("unmarshal tool arguments: %w", err)
		}
	}
	return s.CallTool1(ctx, tool, params)
}

func (s *ServerFeatures) CallTool1(ctx context.Context, tool string, params map[string]any) error {
	if s.Session == nil {
		return ErrNoSession
	}
	result, err := s.Session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tool,
		Arguments: params,
	})
	if err != nil {
		return fmt.Errorf("call tool: %w", err)
	}

	for _, c := range result.Content {
		out, _ := c.MarshalJSON()
		fmt.Fprintln(cmp.Or(s.Out, os.Stdout), string(out))
	}
	return nil
}
