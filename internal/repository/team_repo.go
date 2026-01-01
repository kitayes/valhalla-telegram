package repository

import (
	"valhalla-telegram/internal/domain"

	"gorm.io/gorm"
)

type TeamRepository interface {
	CreateTeam(name string) (*domain.Team, error)
	GetTeamByID(id uint) (*domain.Team, error)
	GetTeamByName(name string) (*domain.Team, error)
	DeleteTeam(id uint) error

	GetAllTeams() ([]domain.Team, error)
	SetCheckIn(teamID uint, status bool) error
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

func (r *teamRepo) GetTeamByName(name string) (*domain.Team, error) {
	var team domain.Team
	err := r.db.Preload("Players").Where("name = ?", name).First(&team).Error
	return &team, err
}

func (r *teamRepo) DeleteTeam(id uint) error {
	return r.db.Delete(&domain.Team{}, id).Error
}

func (r *teamRepo) GetAllTeams() ([]domain.Team, error) {
	var teams []domain.Team
	err := r.db.Preload("Players").Find(&teams).Error
	return teams, err
}

func (r *teamRepo) SetCheckIn(teamID uint, status bool) error {
	return r.db.Model(&domain.Team{}).Where("id = ?", teamID).Update("is_checked_in", status).Error
}
