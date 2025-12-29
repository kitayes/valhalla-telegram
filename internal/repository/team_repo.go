package repository

import (
	"valhalla-telegram/internal/domain"

	"gorm.io/gorm"
)

type TeamRepository interface {
	CreateTeam(name string) (*domain.Team, error)
	GetTeamByID(id uint) (*domain.Team, error)
	DeleteTeam(id uint) error
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

func (r *teamRepo) GetTeamByID(id uint) (*domain.Team, error) {
	var team domain.Team
	err := r.db.First(&team, id).Error
	return &team, err
}

func (r *teamRepo) DeleteTeam(id uint) error {
	return r.db.Unscoped().Delete(&domain.Team{}, id).Error
}
