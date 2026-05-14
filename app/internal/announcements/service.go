package announcements

import (
	"errors"
	"strings"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type Service struct {
	store *SQLiteStore
}

func NewService(store *SQLiteStore) *Service {
	return &Service{store: store}
}

func (s *Service) Create(input UpsertInput) (int64, error) {
	normalized, err := validateUpsert(input)
	if err != nil {
		return 0, err
	}
	return s.store.Create(normalized)
}

func (s *Service) Update(id int64, input UpsertInput) error {
	if id <= 0 {
		return errors.New("invalid announcement id")
	}
	normalized, err := validateUpsert(input)
	if err != nil {
		return err
	}
	return s.store.Update(id, normalized)
}

func (s *Service) List(scope string, userID *types.UserID, active *bool) ([]Announcement, error) {
	return s.store.List(scope, userID, active)
}

func (s *Service) ListActiveForUser(userID types.UserID, now uint64) ([]Announcement, error) {
	return s.store.ListActiveForUser(userID, now)
}

func (s *Service) Delete(id int64) error {
	if id <= 0 {
		return errors.New("invalid announcement id")
	}
	return s.store.Delete(id)
}

func validateUpsert(input UpsertInput) (UpsertInput, error) {
	input.Scope = strings.ToUpper(strings.TrimSpace(input.Scope))
	if input.Scope != ScopeGlobal && input.Scope != ScopeUser {
		return UpsertInput{}, errors.New("invalid scope")
	}
	if input.Scope == ScopeGlobal {
		input.UserID = nil
	}
	if input.Scope == ScopeUser {
		if input.UserID == nil || *input.UserID <= 0 {
			return UpsertInput{}, errors.New("userId is required for USER scope")
		}
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	if input.Title == "" {
		return UpsertInput{}, errors.New("title is required")
	}
	if input.Body == "" {
		return UpsertInput{}, errors.New("body is required")
	}
	if len(input.Title) > 200 {
		return UpsertInput{}, errors.New("title is too long")
	}
	if len(input.Body) > 2000 {
		return UpsertInput{}, errors.New("body is too long")
	}
	if input.StartsAt != nil && input.EndsAt != nil && *input.StartsAt > *input.EndsAt {
		return UpsertInput{}, errors.New("startsAt must be <= endsAt")
	}
	return input, nil
}
