package usecase

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
	"valhalla-telegram/internal/domain"
	"valhalla-telegram/internal/repository"
)

const (
	KbNone   = "empty"
	KbCancel = "cancel"
	KbRole   = "role"
)

type RegistrationUseCase interface {
	RegisterUser(tgID int64, username, firstName string) string
	HandleUserInput(tgID int64, input string) (string, string)

	StartSoloRegistration(tgID int64) (string, string)
	StartTeamRegistration(tgID int64) (string, string)
	StartEditPlayer(tgID int64, slot int) (string, string)
	StartReport(tgID int64) (string, string)

	DeleteTeam(tgID int64) string
	GetTeamInfo(tgID int64) string
	ToggleCheckIn(tgID int64) string

	SetRegistrationOpen(isOpen bool)
	IsRegistrationOpen() bool
	GenerateTeamsCSV() ([]byte, error)
	GetBroadcastList() ([]int64, error)
	AdminDeleteTeam(teamName string) string
	AdminResetUser(tgID int64) string
	HandleReport(tgID int64, photoFileID, caption string) string

	SetTournamentTime(t time.Time)
	GetTournamentTime() time.Time
	GetUncheckedTeams() ([]domain.Team, error)
}

type regUseCase struct {
	playerRepo         repository.PlayerRepository
	teamRepo           repository.TeamRepository
	isRegistrationOpen bool
	tournamentTime     time.Time
}

func NewRegistrationUseCase(pRepo repository.PlayerRepository, tRepo repository.TeamRepository) RegistrationUseCase {
	return &regUseCase{
		playerRepo:         pRepo,
		teamRepo:           tRepo,
		isRegistrationOpen: true,
	}
}

func (uc *regUseCase) RegisterUser(tgID int64, username, firstName string) string {
	idPtr := &tgID
	p := &domain.Player{TelegramID: idPtr, TelegramUsername: username, FirstName: firstName}
	uc.playerRepo.CreateOrUpdate(p)
	return fmt.Sprintf("Привет, %s!", firstName)
}

func (uc *regUseCase) HandleUserInput(tgID int64, input string) (string, string) {
	if input == "Отмена" || input == "/cancel" {
		uc.playerRepo.UpdateState(tgID, domain.StateIdle)
		return "Действие отменено. Возврат в меню.", KbNone
	}

	player, _ := uc.playerRepo.GetByTelegramID(tgID)

	if strings.HasPrefix(player.FSMState, "team_reg_") {
		return uc.handleTeamLoop(player, input)
	}
	if strings.HasPrefix(player.FSMState, "edit_player_") {
		return uc.handleEditLoop(player, input)
	}

	switch player.FSMState {
	case domain.StateWaitingNickname:
		uc.playerRepo.UpdateGameData(tgID, "game_nickname", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingGameID)
		return "Введите ваш Game ID (цифры):", KbCancel

	case domain.StateWaitingGameID:
		uc.playerRepo.UpdateGameData(tgID, "game_id", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingZoneID)
		return "Введите Zone ID (в скобках):", KbCancel

	case domain.StateWaitingZoneID:
		uc.playerRepo.UpdateGameData(tgID, "zone_id", input)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingStars)
		return "Сколько звезд (Rank) в этом сезоне?", KbCancel

	case domain.StateWaitingStars:
		stars, _ := strconv.Atoi(input)
		uc.playerRepo.UpdateGameData(tgID, "stars", stars)
		uc.playerRepo.UpdateState(tgID, domain.StateWaitingRole)
		return "Выберите вашу роль:", KbRole

	case domain.StateWaitingRole:
		uc.playerRepo.UpdateGameData(tgID, "main_role", input)
		uc.playerRepo.UpdateState(tgID, domain.StateIdle)
		return "Соло-регистрация завершена!", KbNone

	case domain.StateWaitingTeamName:
		team, err := uc.teamRepo.CreateTeam(input)
		if err != nil {
			return "Это имя занято, попробуйте другое:", KbCancel
		}
		uc.playerRepo.UpdateGameData(tgID, "team_id", team.ID)
		uc.playerRepo.UpdateGameData(tgID, "is_captain", true)
		uc.playerRepo.UpdateState(tgID, "team_reg_nick_1")
		return fmt.Sprintf("Команда '%s' создана!\n\n--- Игрок №1 (Капитан) ---\nВведите ваш Ник:", input), KbCancel

	default:
		return "Используйте меню для управления.", KbNone
	}
}

func (uc *regUseCase) handleTeamLoop(captain *domain.Player, input string) (string, string) {
	parts := strings.Split(captain.FSMState, "_")
	step := parts[2]
	slot, _ := strconv.Atoi(parts[3])
	teamID := *captain.TeamID
	captainID := *captain.TelegramID
	isCapSlot := slot == 1

	switch step {
	case "nick":
		if isCapSlot {
			uc.playerRepo.UpdateGameData(captainID, "game_nickname", input)
		} else {
			newP := &domain.Player{TeamID: &teamID, GameNickname: input, IsSubstitute: slot >= 6}
			uc.playerRepo.CreateTeammate(newP)
		}
		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_id_%d", slot))
		return "Введите Game ID:", KbCancel

	case "id":
		if isCapSlot {
			uc.playerRepo.UpdateGameData(captainID, "game_id", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "game_id", input)
		}
		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_zone_%d", slot))
		return "Введите Zone ID:", KbCancel

	case "zone":
		if isCapSlot {
			uc.playerRepo.UpdateGameData(captainID, "zone_id", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "zone_id", input)
		}
		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_rank_%d", slot))
		return "Кол-во звезд (Rank):", KbCancel

	case "rank":
		stars, _ := strconv.Atoi(input)
		if isCapSlot {
			uc.playerRepo.UpdateGameData(captainID, "stars", stars)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "stars", stars)
		}
		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_role_%d", slot))
		return "Выберите роль:", KbRole

	case "role":
		if isCapSlot {
			uc.playerRepo.UpdateGameData(captainID, "main_role", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "main_role", input)
		}
		uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_contact_%d", slot))
		return "Telegram контакт (например @user или '-'):", KbCancel

	case "contact":
		if isCapSlot {
			uc.playerRepo.UpdateGameData(captainID, "telegram_username", input)
		} else {
			uc.playerRepo.UpdateLastTeammateData(teamID, "telegram_username", input)
		}

		if slot < 7 {
			next := slot + 1
			uc.playerRepo.UpdateState(captainID, fmt.Sprintf("team_reg_nick_%d", next))
			return fmt.Sprintf("✅ Игрок %d готов.\n\n--- Игрок №%d ---\nВведите Ник:", slot, next), KbCancel
		}
		uc.playerRepo.UpdateState(captainID, domain.StateIdle)
		return "Регистрация команды завершена!", KbNone
	}
	return "Ошибка.", KbNone
}

func (uc *regUseCase) handleEditLoop(captain *domain.Player, input string) (string, string) {
	parts := strings.Split(captain.FSMState, "_")
	step := parts[2]
	slot, _ := strconv.Atoi(parts[3])
	members, _ := uc.playerRepo.GetTeamMembers(*captain.TeamID)

	if slot > len(members) {
		uc.playerRepo.UpdateState(*captain.TelegramID, domain.StateIdle)
		return "Игрок не найден.", KbNone
	}
	targetID := members[slot-1].ID

	switch step {
	case "nick":
		uc.playerRepo.UpdatePlayerField(targetID, "game_nickname", input)
		uc.playerRepo.UpdateState(*captain.TelegramID, fmt.Sprintf("edit_player_id_%d", slot))
		return "Ник изменен. Введите Game ID:", KbCancel
	case "id":
		uc.playerRepo.UpdatePlayerField(targetID, "game_id", input)
		uc.playerRepo.UpdateState(*captain.TelegramID, fmt.Sprintf("edit_player_role_%d", slot))
		return "ID изменен. Выберите роль:", KbRole
	case "role":
		uc.playerRepo.UpdatePlayerField(targetID, "main_role", input)
		uc.playerRepo.UpdateState(*captain.TelegramID, domain.StateIdle)
		return "Данные обновлены!", KbNone
	}
	return "Ошибка.", KbNone
}

func (uc *regUseCase) StartSoloRegistration(tgID int64) (string, string) {
	if !uc.isRegistrationOpen {
		return "Регистрация закрыта.", KbNone
	}
	uc.playerRepo.UpdateState(tgID, domain.StateWaitingNickname)
	return "Начинаем соло-регистрацию. Введите Ник:", KbCancel
}

func (uc *regUseCase) StartTeamRegistration(tgID int64) (string, string) {
	if !uc.isRegistrationOpen {
		return "Регистрация закрыта.", KbNone
	}
	uc.playerRepo.UpdateState(tgID, domain.StateWaitingTeamName)
	return "Введите Название команды:", KbCancel
}

func (uc *regUseCase) StartEditPlayer(tgID int64, slot int) (string, string) {
	uc.playerRepo.UpdateState(tgID, fmt.Sprintf("edit_player_nick_%d", slot))
	return fmt.Sprintf("Редактируем игрока %d. Введите новый Ник:", slot), KbCancel
}

func (uc *regUseCase) StartReport(tgID int64) (string, string) {
	uc.playerRepo.UpdateState(tgID, domain.StateWaitingReport)
	return "Отправьте скриншот результата матча:", KbCancel
}

func (uc *regUseCase) GetTeamInfo(tgID int64) string {
	p, _ := uc.playerRepo.GetByTelegramID(tgID)
	if p.TeamID == nil {
		return "Вы не в команде."
	}
	team, _ := uc.teamRepo.GetTeamByID(*p.TeamID)
	members, _ := uc.playerRepo.GetTeamMembers(*p.TeamID)

	status := "Не подтверждена"
	if team.IsCheckedIn {
		status = "Подтверждена"
	}

	res := fmt.Sprintf("Команда: %s\nСтатус: %s\n\n", team.Name, status)
	for i, m := range members {
		res += fmt.Sprintf("%d. %s (%s)\n   ID: %s (%s)\n\n", i+1, m.GameNickname, m.MainRole, m.GameID, m.ZoneID)
	}
	return res
}

func (uc *regUseCase) ToggleCheckIn(tgID int64) string {
	p, _ := uc.playerRepo.GetByTelegramID(tgID)
	if p.TeamID == nil || !p.IsCaptain {
		return "Только капитан может делать Check-in."
	}
	t, _ := uc.teamRepo.GetTeamByID(*p.TeamID)
	uc.teamRepo.SetCheckIn(t.ID, !t.IsCheckedIn)
	return "Статус Check-in изменен."
}

func (uc *regUseCase) DeleteTeam(tgID int64) string {
	p, _ := uc.playerRepo.GetByTelegramID(tgID)
	if p.TeamID == nil || !p.IsCaptain {
		return "Только капитан может удалить команду."
	}
	id := *p.TeamID
	uc.playerRepo.ResetTeamID(id)
	uc.teamRepo.DeleteTeam(id)
	return "Команда удалена."
}

func (uc *regUseCase) SetRegistrationOpen(isOpen bool) { uc.isRegistrationOpen = isOpen }
func (uc *regUseCase) IsRegistrationOpen() bool        { return uc.isRegistrationOpen }

func (uc *regUseCase) AdminDeleteTeam(name string) string {
	t, err := uc.teamRepo.GetTeamByName(name)
	if err != nil {
		return "Не найдена."
	}
	uc.playerRepo.ResetTeamID(t.ID)
	uc.teamRepo.DeleteTeam(t.ID)
	return "Удалена."
}

func (uc *regUseCase) AdminResetUser(id int64) string {
	uc.playerRepo.UpdateState(id, domain.StateIdle)
	return "Сброшен."
}

func (uc *regUseCase) GetBroadcastList() ([]int64, error) {
	caps, err := uc.playerRepo.GetAllCaptains()
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, c := range caps {
		if c.TelegramID != nil {
			ids = append(ids, *c.TelegramID)
		}
	}
	return ids, nil
}

func (uc *regUseCase) GenerateTeamsCSV() ([]byte, error) {
	teams, err := uc.teamRepo.GetAllTeams()
	if err != nil {
		return nil, err
	}
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)
	w.Write([]string{"Team", "CheckIn", "Nick", "ID", "Zone", "Role"})
	for _, t := range teams {
		for _, m := range t.Players {
			w.Write([]string{t.Name, strconv.FormatBool(t.IsCheckedIn), m.GameNickname, m.GameID, m.ZoneID, string(m.MainRole)})
		}
	}
	w.Flush()
	return b.Bytes(), nil
}

func (uc *regUseCase) HandleReport(tgID int64, fileID, caption string) string {
	p, _ := uc.playerRepo.GetByTelegramID(tgID)
	if p.FSMState != domain.StateWaitingReport {
		return "Используйте /report"
	}
	t, _ := uc.teamRepo.GetTeamByID(*p.TeamID)
	uc.playerRepo.UpdateState(tgID, domain.StateIdle)
	return fmt.Sprintf("ADMIN_REPORT:%s:Команда: %s\nКапитан: @%s\nИнфо: %s", fileID, t.Name, p.TelegramUsername, caption)
}

func (uc *regUseCase) SetTournamentTime(t time.Time) { uc.tournamentTime = t }
func (uc *regUseCase) GetTournamentTime() time.Time  { return uc.tournamentTime }

func (uc *regUseCase) GetUncheckedTeams() ([]domain.Team, error) {
	allTeams, err := uc.teamRepo.GetAllTeams()
	if err != nil {
		return nil, err
	}
	var unchecked []domain.Team
	for _, t := range allTeams {
		if !t.IsCheckedIn {
			unchecked = append(unchecked, t)
		}
	}
	return unchecked, nil
}
