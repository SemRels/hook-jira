// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package grpc

import (
	"context"

	semrelplugin "github.com/SemRels/hook-jira/internal/plugin"
)

// HealthResponse is a lightweight stand-in until generated protobuf bindings are wired in.
type HealthResponse struct {
	Name     string
	Contexts []string
}

// ProviderServer adapts a provider implementation for the future gRPC transport layer.
type ProviderServer struct {
	provider semrelplugin.Provider
}

func NewProviderServer(provider semrelplugin.Provider) *ProviderServer {
	return &ProviderServer{provider: provider}
}

func (s *ProviderServer) Health(ctx context.Context) (*HealthResponse, error) {
	if err := s.provider.HealthCheck(ctx); err != nil {
		return nil, err
	}

	return &HealthResponse{Name: s.provider.Name(), Contexts: s.provider.ReleaseContext()}, nil
}
