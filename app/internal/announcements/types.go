package announcements

import "github.com/maxonlinux/meta-terminal-go/pkg/types"

const (
	ScopeGlobal = "GLOBAL"
	ScopeUser   = "USER"
)

type Announcement struct {
	ID        int64
	Scope     string
	UserID    *types.UserID
	Title     string
	Body      string
	Link      *string
	Priority  int
	IsActive  bool
	StartsAt  *uint64
	EndsAt    *uint64
	CreatedAt uint64
	UpdatedAt uint64
}

type UpsertInput struct {
	Scope    string
	UserID   *types.UserID
	Title    string
	Body     string
	Link     *string
	Priority int
	IsActive bool
	StartsAt *uint64
	EndsAt   *uint64
}
