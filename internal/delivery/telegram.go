package delivery

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"valhalla-telegram/internal/usecase"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var adminIDs = []int64{
	8150393380,
	6498318881,
	1209165513,
	5306796711,
}

func isAdmin(id int64) bool {
	for _, admin := range adminIDs {
		if admin == id {
			return true
		}
	}
	return false
}

type TelegramHandler struct {
	bot     *tgbotapi.BotAPI
	useCase usecase.RegistrationUseCase
}

func NewTelegramHandler(token string, uc usecase.RegistrationUseCase) (*TelegramHandler, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	return &TelegramHandler{bot: bot, useCase: uc}, nil
}

func (h *TelegramHandler) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		msg := update.Message
		chatID := msg.Chat.ID
		text := msg.Text
		user := msg.From

		if msg.Photo != nil && len(msg.Photo) > 0 {
			photoID := msg.Photo[len(msg.Photo)-1].FileID
			caption := msg.Caption

			resp := h.useCase.HandleReport(chatID, photoID, caption)

			if strings.HasPrefix(resp, "ADMIN_REPORT:") {
				parts := strings.SplitN(resp, ":", 3)
				if len(parts) == 3 {
					fileID := parts[1]
					reportText := parts[2]

					count := 0
					for _, adminID := range adminIDs {
						photoMsg := tgbotapi.NewPhoto(adminID, tgbotapi.FileID(fileID))
						photoMsg.Caption = "📨 НОВЫЙ РЕПОРТ ОТ КОМАНДЫ:\n\n" + reportText
						_, err := h.bot.Send(photoMsg)
						if err == nil {
							count++
						}
					}
					h.sendMessage(chatID, fmt.Sprintf("✅ Результат отправлен %d администраторам! Ожидайте подтверждения.", count), false)
				}
			} else {
				h.sendMessage(chatID, resp, false)
			}
			continue
		}

		h.useCase.RegisterUser(chatID, user.UserName, user.FirstName)

		var response string
		var showKeyboard bool

		if isAdmin(chatID) {
			if strings.HasPrefix(text, "/admin") {
				response = "👮 Админ-панель:\n\n" +
					"/export - Скачать список команд (Excel/CSV)\n" +
					"/broadcast [текст] - Рассылка всем капитанам\n" +
					"/close_reg - Закрыть регистрацию\n" +
					"/open_reg - Открыть регистрацию\n" +
					"/del_team [Название] - Удалить команду\n" +
					"/reset_user [ChatID] - Сброс FSM"
				h.sendMessage(chatID, response, false)
				continue
			}

			if text == "/export" {
				csvData, err := h.useCase.GenerateTeamsCSV()
				if err != nil {
					h.sendMessage(chatID, "Ошибка генерации: "+err.Error(), false)
				} else {
					fileBytes := tgbotapi.FileBytes{
						Name:  "teams_export.csv",
						Bytes: csvData,
					}
					docMsg := tgbotapi.NewDocument(chatID, fileBytes)
					h.bot.Send(docMsg)
				}
				continue
			}

			if strings.HasPrefix(text, "/broadcast ") {
				msgText := strings.TrimPrefix(text, "/broadcast ")
				captains, _ := h.useCase.GetBroadcastList()

				count := 0
				for _, capID := range captains {
					h.sendMessage(capID, "ОФИЦИАЛЬНОЕ ОБЪЯВЛЕНИЕ:\n\n"+msgText, false)
					count++
				}
				h.sendMessage(chatID, "Рассылка завершена. "+strconv.Itoa(count)+" капитанов получили сообщение.", false)
				continue
			}

			if text == "/close_reg" {
				h.useCase.SetRegistrationOpen(false)
				h.sendMessage(chatID, "Регистрация закрыта.", false)
				continue
			}
			if text == "/open_reg" {
				h.useCase.SetRegistrationOpen(true)
				h.sendMessage(chatID, "Регистрация открыта.", false)
				continue
			}

			if strings.HasPrefix(text, "/del_team ") {
				teamName := strings.TrimPrefix(text, "/del_team ")
				resp := h.useCase.AdminDeleteTeam(teamName)
				h.sendMessage(chatID, resp, false)
				continue
			}

			if strings.HasPrefix(text, "/reset_user ") {
				targetIDStr := strings.TrimPrefix(text, "/reset_user ")
				targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
				if err != nil {
					h.sendMessage(chatID, "ID должен быть числом.", false)
				} else {
					resp := h.useCase.AdminResetUser(targetID)
					h.sendMessage(chatID, resp, false)
				}
				continue
			}
		}

		if strings.HasPrefix(text, "/edit_player") {
			parts := strings.Fields(text)
			if len(parts) != 2 {
				response = "Используйте: /edit_player [номер]\nПример: /edit_player 3"
			} else {
				slot, err := strconv.Atoi(parts[1])
				if err != nil {
					response = "Номер игрока должен быть числом."
				} else {
					response = h.useCase.StartEditPlayer(chatID, slot)
				}
			}
			h.sendMessage(chatID, response, false)
			continue
		}

		switch text {
		case "/start":
			response = "Valhalla Cup Bot\n\n" +
				"/reg_solo - Ищу команду\n" +
				"/reg_team - Создать команду\n" +
				"/my_team - Мой состав\n" +
				"/edit_player [№] - Изменить игрока\n" +
				"/checkin - Подтвердить участие\n" +
				"/report - Отправить результат матча\n" +
				"/delete_team - Удалить команду"

		case "/reg_solo":
			response = h.useCase.StartSoloRegistration(chatID)
		case "/reg_team":
			response = h.useCase.StartTeamRegistration(chatID)
		case "/my_team":
			response = h.useCase.GetTeamInfo(chatID)
		case "/checkin":
			response = h.useCase.ToggleCheckIn(chatID)
		case "/delete_team":
			response = h.useCase.DeleteTeam(chatID)
		case "/report":
			response = h.useCase.StartReport(chatID)

		default:
			response, showKeyboard = h.useCase.HandleUserInput(chatID, text)
		}

		h.sendMessage(chatID, response, showKeyboard)
	}
}

func (h *TelegramHandler) sendMessage(chatID int64, text string, showKeyboard bool) {
	if text == "" {
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)

	if showKeyboard {
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Gold"),
				tgbotapi.NewKeyboardButton("Exp"),
				tgbotapi.NewKeyboardButton("Mid"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Roam"),
				tgbotapi.NewKeyboardButton("Jungle"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Замена"),
				tgbotapi.NewKeyboardButton("Любая"),
			),
		)
		msg.ReplyMarkup = keyboard
	} else {
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	}

	h.bot.Send(msg)
}
