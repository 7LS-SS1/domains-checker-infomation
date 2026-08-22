package sheets

import (
	"context"

	"github.com/google/uuid"
)

type AccessTokenProvider interface {
	AccessToken(context.Context, uuid.UUID) (string, error)
}

type ConnectedSource struct {
	Google *GoogleClient
	Tokens AccessTokenProvider
}

func (s ConnectedSource) Fetch(ctx context.Context, config Config) (Snapshot, error) {
	if config.ConnectionID == nil {
		return s.Google.Fetch(ctx, config)
	}
	if s.Tokens == nil {
		return Snapshot{}, ErrCredentials
	}
	token, err := s.Tokens.AccessToken(ctx, *config.ConnectionID)
	if err != nil {
		return Snapshot{}, err
	}
	return s.Google.FetchWithAccessToken(ctx, config, token)
}
