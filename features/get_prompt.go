package features

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *ServerFeatures) GetPrompt(ctx context.Context, prompt, data string) error {
	params := map[string]string{}
	if data != "" {
		if err := json.Unmarshal([]byte(data), &params); err != nil {
			return fmt.Errorf("unmarshal prompt arguments: %w", err)
		}
	}
	return s.GetPrompt1(ctx, prompt, params)
}

func (s *ServerFeatures) GetPrompt1(ctx context.Context, prompt string, params map[string]string) error {
	if s.Session == nil {
		return ErrNoSession
	}
	result, err := s.Session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      prompt,
		Arguments: params,
	})
	if err != nil {
		return fmt.Errorf("get prompt: %w", err)
	}

	for _, c := range result.Messages {
		json.NewEncoder(cmp.Or(s.Out, os.Stdout)).Encode(c)
	}
	return nil
}
