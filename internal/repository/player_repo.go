package repository

import (
	"gorm.io/gorm"
	"valhalla-telegram/internal/domain"
)

type PlayerRepository interface {
	GetByTelegramID(tgID int64) (*domain.Player, error)
	CreateOrUpdate(player *domain.Player) error
	UpdateState(tgID int64, state string) error
	UpdateGameData(tgID int64, column string, value interface{}) error
}

type playerRepo struct {
	db *gorm.DB
}

func NewPlayerRepository(db *gorm.DB) PlayerRepository {
	return &playerRepo{db: db}
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

func (r *playerRepo) UpdateState(tgID int64, state string) error {
	return r.db.Model(&domain.Player{}).Where("telegram_id = ?", tgID).Update("fsm_state", state).Error
}

func (r *playerRepo) UpdateGameData(tgID int64, column string, value interface{}) error {
	return r.db.Model(&domain.Player{}).Where("telegram_id = ?", tgID).Update(column, value).Error
}
