package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Group represents a group within an organization or a standalone group
type Group struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	OrganizationID *uint          `gorm:"index" json:"organization_id,omitempty"` // nullable for standalone groups
	Name           string         `gorm:"not null" json:"name"`
	Description    *string        `json:"description,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Roles        []GroupRole   `gorm:"foreignKey:GroupID" json:"roles,omitempty"`
	Members      []GroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

// TableName overrides the table name
func (Group) TableName() string {
	return "groups"
}

// GroupRole represents a role within a group with dynamic permissions
type GroupRole struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	GroupID     uint           `gorm:"not null;index" json:"group_id"`
	Name        string         `gorm:"not null" json:"name"`
	Permissions pq.StringArray `gorm:"type:text[];default:'{}'" json:"permissions"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Group   Group         `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	Members []GroupMember `gorm:"foreignKey:RoleID" json:"members,omitempty"`
}

// TableName overrides the table name
func (GroupRole) TableName() string {
	return "group_roles"
}

// GroupMember represents the membership relationship between users and groups
type GroupMember struct {
	UserID    uint           `gorm:"primaryKey;not null" json:"user_id"`
	GroupID   uint           `gorm:"primaryKey;not null" json:"group_id"`
	RoleID    uint           `gorm:"not null;index" json:"role_id"`
	JoinedAt  time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"joined_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User  User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Group Group     `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	Role  GroupRole `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

// TableName overrides the table name
func (GroupMember) TableName() string {
	return "group_members"
}

// Permission constants for group-level permissions
const (
	PermDocRead      = "DOC_READ"
	PermDocUpload    = "DOC_UPLOAD"
	PermDocDelete    = "DOC_DELETE"
	PermDocEdit      = "DOC_EDIT"
	PermChatAccess   = "CHAT_ACCESS"
	PermMemberAdd    = "MEMBER_ADD"
	PermMemberRemove = "MEMBER_REMOVE"
	PermRoleManage   = "ROLE_MANAGE"
	PermGroupAdmin   = "GROUP_ADMIN"
)

// HasPermission checks if the role has a specific permission
func (r *GroupRole) HasPermission(permission string) bool {
	for _, p := range r.Permissions {
		if p == permission || p == PermGroupAdmin {
			return true
		}
	}
	return false
}
