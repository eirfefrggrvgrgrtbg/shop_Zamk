package search

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrQueryTooShort = errors.New("search query too short")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GlobalSearch(ctx context.Context, rawQuery string, perms AllowedPermissions) ([]GlobalSearchResult, error) {
	q := strings.TrimSpace(rawQuery)
	if utf8.RuneCountInString(q) < 2 {
		return nil, ErrQueryTooShort
	}
	return s.repo.GlobalSearch(ctx, q, perms)
}
