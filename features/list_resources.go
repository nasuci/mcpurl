package features

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s ServerFeatures) ListResources(ctx context.Context) ([]*mcp.Resource, error) {
	if s.Session == nil {
		return nil, ErrNoSession
	}
	params := &mcp.ListResourcesParams{}
	var resources []*mcp.Resource
	for {
		result, err := s.Session.ListResources(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list resources: %w", err)
		}
		resources = append(resources, result.Resources...)
		if result.NextCursor == "" {
			break
		}
		params.Cursor = result.NextCursor
	}
	return resources, nil
}

func (s *ServerFeatures) PrintResources(ctx context.Context) error {
	resources, err := s.ListResources(ctx)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		json.NewEncoder(cmp.Or(s.Out, os.Stdout)).Encode(resource)
	}
	return nil
}
