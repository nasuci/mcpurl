package features

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s ServerFeatures) ListPrompts(ctx context.Context) ([]*mcp.Prompt, error) {
	if s.Session == nil {
		return nil, ErrNoSession
	}
	params := &mcp.ListPromptsParams{}
	var prompts []*mcp.Prompt
	for {
		result, err := s.Session.ListPrompts(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list prompts: %w", err)
		}
		prompts = append(prompts, result.Prompts...)
		if result.NextCursor == "" {
			break
		}
		params.Cursor = result.NextCursor
	}
	return prompts, nil
}

func (s *ServerFeatures) PrintPrompts(ctx context.Context) error {
	prompts, err := s.ListPrompts(ctx)
	if err != nil {
		return err
	}
	for _, prompt := range prompts {
		json.NewEncoder(cmp.Or(s.Out, os.Stdout)).Encode(prompt)
	}
	return nil
}
