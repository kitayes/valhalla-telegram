package domain

import (
	"time"

	"gorm.io/gorm"
)

const (
	StateIdle          = ""
	StateWaitingGameID = "waiting_game_id"
	StateWaitingZoneID = "waiting_zone_id"
	StateWaitingStars  = "waiting_stars"
	StateWaitingRole   = "waiting_role"
)

type Role string

const (
	RoleGold   Role = "Gold"
	RoleExp    Role = "Exp"
	RoleMid    Role = "Mid"
	RoleRoam   Role = "Roam"
	RoleJungle Role = "Jungle"
)

type Team struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name    string   `gorm:"unique;not null"`
	Players []Player `gorm:"foreignKey:TeamID"`
}

type Player struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	TelegramID       int64  `gorm:"uniqueIndex;not null"`
	TelegramUsername string `gorm:"size:64"`
	FirstName        string `gorm:"size:64"`

	GameNickname string `gorm:"size:64"`
	GameID       string `gorm:"size:32"`
	ZoneID       string `gorm:"size:10"`
	Stars        int    `gorm:"default:0"`
	MainRole     Role   `gorm:"size:20"`

	FSMState string `gorm:"size:32"`

	TeamID *uint
}
