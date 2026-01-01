package domain

import (
	"time"

	"gorm.io/gorm"
)

const (
	StateIdle            = ""
	StateWaitingNickname = "waiting_nickname"
	StateWaitingGameID   = "waiting_game_id"
	StateWaitingZoneID   = "waiting_zone_id"
	StateWaitingStars    = "waiting_stars"
	StateWaitingRole     = "waiting_role"
	StateWaitingTeamName = "waiting_team_name"
	StateWaitingReport   = "waiting_report"
)

type Role string

const (
	RoleGold   Role = "Gold"
	RoleExp    Role = "Exp"
	RoleMid    Role = "Mid"
	RoleRoam   Role = "Roam"
	RoleJungle Role = "Jungle"
	RoleSub    Role = "Замена"
	RoleAny    Role = "Любая"
)

type Team struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name        string   `gorm:"unique;not null"`
	IsCheckedIn bool     `gorm:"default:false"`
	Players     []Player `gorm:"foreignKey:TeamID"`
}

type Player struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	TelegramID       *int64 `gorm:"uniqueIndex"`
	TelegramUsername string `gorm:"size:64"`
	FirstName        string `gorm:"size:64"`

	GameNickname string `gorm:"size:64"`
	GameID       string `gorm:"size:32"`
	ZoneID       string `gorm:"size:10"`
	Stars        int    `gorm:"default:0"`
	MainRole     Role   `gorm:"size:20"`

	IsCaptain    bool `gorm:"default:false"`
	IsSubstitute bool `gorm:"default:false"`

	FSMState string `gorm:"size:64"`

	TeamID *uint
}
