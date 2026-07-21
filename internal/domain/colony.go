package domain

import "time"

type ColonyMemberRole string

const (
	ColonyRoleOwner  ColonyMemberRole = "OWNER"
	ColonyRoleMember ColonyMemberRole = "MEMBER"
)

type Colony struct {
	ID             string
	Name           string
	Description    string
	OwnerUserID    string
	InviteCodeHash []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ColonyMember struct {
	ColonyID string
	UserID   string
	Role     ColonyMemberRole
	JoinedAt time.Time
}

type SharedItem struct {
	ID            string
	ColonyID      string
	SourceType    string
	SourceID      string
	CreatedBy     string
	TitleSnapshot string
	DateSnapshot  *time.Time
	CreatedAt     time.Time
}
