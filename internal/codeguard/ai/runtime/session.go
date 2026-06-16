package runtime

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type Session struct {
	enabled  bool
	provider Provider
	cache    *Cache
}

func NewSession(cfg core.AIConfig, opts core.ScanOptions) (*Session, error) {
	enabled := opts.EnableAI || (cfg.Enabled != nil && *cfg.Enabled)
	if !enabled {
		return &Session{}, nil
	}
	provider, available, err := BuildProvider(cfg.Provider)
	if err != nil {
		return nil, err
	}
	if !available {
		return &Session{enabled: false}, nil
	}
	return &Session{
		enabled:  true,
		provider: provider,
		cache:    LoadCache(cfg.Cache.Path),
	}, nil
}

func (s *Session) Enabled() bool {
	return s != nil && s.enabled && s.provider != nil
}

func (s *Session) ProviderName() string {
	if s == nil || s.provider == nil {
		return ""
	}
	return s.provider.Name()
}

func (s *Session) Save() error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.Save()
}

func (s *Session) EvaluateCached(ctx context.Context, req Request) (Response, string, error) {
	key, contentHash := CacheKey(req)
	if s.cache != nil {
		if cached, ok := s.cache.Get(key); ok && cached.ContentHash == contentHash {
			return Response{Raw: cached.Raw}, contentHash, nil
		}
	}
	resp, err := s.provider.Evaluate(ctx, req)
	if err != nil {
		return Response{}, contentHash, err
	}
	if s.cache != nil {
		s.cache.Put(key, CachedVerdict{
			Kind:        req.Kind,
			ContentHash: contentHash,
			Raw:         resp.Raw,
		})
	}
	return resp, contentHash, nil
}

func CacheKey(req Request) (string, string) {
	content := strings.Join([]string{req.Kind, req.System, req.Prompt, req.InputJSON}, "|")
	sum := sha1.Sum([]byte(content))
	hash := hex.EncodeToString(sum[:])
	return req.Kind + "|" + hash, hash
}
