package player

import (
	"context"
	"errors"
)

var errCommentAccountChanged = errors.New("账号状态已变化，请重新打开评论")

// Snapshot credentials and register cancellation under mu; never hold mu over
// comment or token-refresh network traffic. An account change invalidates both
// responses and refresh commits, so an in-flight request cannot restore a logout.
func (s *Service) requestComments(parent context.Context, body object) (any, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	s.mu.Lock()
	access, refresh, err := s.tokens()
	epoch := s.accountEpoch
	s.commentSeq++
	id := s.commentSeq
	if s.commentCancels == nil {
		s.commentCancels = map[uint64]context.CancelFunc{}
	}
	s.commentCancels[id] = cancel
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.commentCancels, id); s.mu.Unlock() }()
	if err != nil {
		return nil, err
	}
	if access == "" {
		return nil, errLogin
	}
	load := func(token string) (any, error) {
		return s.client.ListComments(ctx, token, str(body["eid"]), str(body["thread"]), body["loadMoreKey"], str(body["order"]))
	}
	v, err := load(access)
	if unauthorized(err) && refresh != "" {
		access, err = s.refreshCommentToken(ctx, epoch, access, refresh)
		if err == nil {
			v, err = load(access)
		}
	}
	s.mu.Lock()
	changed := s.accountEpoch != epoch
	s.mu.Unlock()
	if changed {
		return nil, errCommentAccountChanged
	}
	if err != nil {
		return nil, err
	}
	return merged(v), nil
}

func (s *Service) refreshCommentToken(ctx context.Context, epoch uint64, access, refresh string) (string, error) {
	// Multiple comment pages may encounter the same expired access token.
	s.commentRefreshMu.Lock()
	defer s.commentRefreshMu.Unlock()
	s.mu.Lock()
	current, currentRefresh, err := s.tokens()
	changed := s.accountEpoch != epoch
	s.mu.Unlock()
	if changed {
		return "", errCommentAccountChanged
	}
	if err != nil {
		return "", err
	}
	if current == "" {
		return "", errLogin
	}
	if current != access || currentRefresh != refresh {
		return current, nil
	}
	fresh, err := s.client.RefreshTokens(ctx, access, refresh)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accountEpoch != epoch {
		return "", errCommentAccountChanged
	}
	current, currentRefresh, err = s.tokens()
	if err != nil {
		return "", err
	}
	if current == "" {
		return "", errLogin
	}
	// Another endpoint may have refreshed while the network call was running.
	if current != access || currentRefresh != refresh {
		return current, nil
	}
	if err = s.writeTokens(fresh.AccessToken, fresh.RefreshToken); err != nil {
		return "", err
	}
	return fresh.AccessToken, nil
}
