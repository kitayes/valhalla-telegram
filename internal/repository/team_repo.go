package repository

import (
	"valhalla-telegram/internal/domain"

	"gorm.io/gorm"
)

type TeamRepository interface {
	CreateTeam(name string) (*domain.Team, error)
}

type teamRepo struct {
	db *gorm.DB
}

func NewTeamRepository(db *gorm.DB) TeamRepository {
	return &teamRepo{db: db}
}

func (r *teamRepo) CreateTeam(name string) (*domain.Team, error) {
	team := &domain.Team{Name: name}
	err := r.db.Create(team).Error
	return team, err
}
