package repository

import (
	"valhalla-telegram/internal/domain"

	"gorm.io/gorm"
)

type PlayerRepository interface {
	GetByTelegramID(tgID int64) (*domain.Player, error)
	CreateOrUpdate(player *domain.Player) error
	CreateTeammate(player *domain.Player) error // Новый метод для тиммейтов

	UpdateState(tgID int64, state string) error
	UpdateGameData(tgID int64, column string, value interface{}) error

	ResetTeamID(teamID uint) error
	GetTeamMembers(teamID uint) ([]domain.Player, error)

	UpdateLastTeammateData(teamID uint, column string, value interface{}) error

	GetAllCaptains() ([]domain.Player, error)

	UpdatePlayerField(playerID uint, column string, value interface{}) error
}

type playerRepo struct {
	db *gorm.DB
}

func NewPlayerRepository(db *gorm.DB) PlayerRepository {
	return &playerRepo{db: db}
}

func (r *playerRepo) ResetTeamID(teamID uint) error {
	return r.db.Model(&domain.Player{}).Where("team_id = ?", teamID).Update("team_id", nil).Error
}

func (r *playerRepo) GetTeamMembers(teamID uint) ([]domain.Player, error) {
	var players []domain.Player
	err := r.db.Where("team_id = ?", teamID).Order("id asc").Find(&players).Error
	return players, err
}

func (r *playerRepo) GetByTelegramID(tgID int64) (*domain.Player, error) {
	var p domain.Player
	err := r.db.Where("telegram_id = ?", tgID).First(&p).Error
	return &p, err
}

func (r *playerRepo) CreateOrUpdate(player *domain.Player) error {
	return r.db.Where(domain.Player{TelegramID: player.TelegramID}).
		Assign(domain.Player{
			TelegramUsername: player.TelegramUsername,
			FirstName:        player.FirstName,
		}).
		FirstOrCreate(player).Error
}

func (r *playerRepo) CreateTeammate(player *domain.Player) error {
	return r.db.Create(player).Error
}

func (r *playerRepo) UpdateState(tgID int64, state string) error {
	return r.db.Model(&domain.Player{}).Where("telegram_id = ?", tgID).Update("fsm_state", state).Error
}

func (r *playerRepo) UpdateGameData(tgID int64, column string, value interface{}) error {
	return r.db.Model(&domain.Player{}).Where("telegram_id = ?", tgID).Update(column, value).Error
}

func (r *playerRepo) UpdateLastTeammateData(teamID uint, column string, value interface{}) error {
	var p domain.Player
	if err := r.db.Where("team_id = ?", teamID).Order("id desc").First(&p).Error; err != nil {
		return err
	}
	return r.db.Model(&p).Update(column, value).Error
}

func (r *playerRepo) GetAllCaptains() ([]domain.Player, error) {
	var captains []domain.Player
	err := r.db.Where("is_captain = ? AND telegram_id IS NOT NULL", true).Find(&captains).Error
	return captains, err
}

func (r *playerRepo) UpdatePlayerField(playerID uint, column string, value interface{}) error {
	return r.db.Model(&domain.Player{}).Where("id = ?", playerID).Update(column, value).Error
}
