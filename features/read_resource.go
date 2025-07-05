package features

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *ServerFeatures) ReadResource(ctx context.Context, resource string) error {
	if s.Session == nil {
		return ErrNoSession
	}
	result, err := s.Session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: resource,
	})
	if err != nil {
		return fmt.Errorf("read resource: %w", err)
	}
	for _, c := range result.Contents {
		json.NewEncoder(cmp.Or(s.Out, os.Stdout)).Encode(c)
	}
	return nil
}
