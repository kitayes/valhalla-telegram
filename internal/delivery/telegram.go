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

					for _, adminID := range adminIDs {
						photoMsg := tgbotapi.NewPhoto(adminID, tgbotapi.FileID(fileID))
						photoMsg.Caption = "НОВЫЙ РЕЗУЛЬТАТ МАТЧА:\n\n" + reportText
						h.bot.Send(photoMsg)
					}
					h.sendMessage(chatID, "Скриншот отправлен судьям!", "empty")
				}
			} else {
				h.sendMessage(chatID, resp, "empty")
			}
			continue
		}

		h.useCase.RegisterUser(chatID, user.UserName, user.FirstName)

		var response string
		var kbType string = "empty"

		if isAdmin(chatID) {
			if strings.HasPrefix(text, "/admin") {
				response = "Админ-панель:\n\n" +
					"/export - Список команд (CSV)\n" +
					"/broadcast [текст] - Рассылка капитанам\n" +
					"/close_reg - Закрыть регистрацию\n" +
					"/open_reg - Открыть регистрацию\n" +
					"/del_team [Название] - Удалить команду\n" +
					"/reset_user [ID] - Сбросить FSM"
				h.sendMessage(chatID, response, "empty")
				continue
			}

			if text == "/export" {
				csvData, err := h.useCase.GenerateTeamsCSV()
				if err != nil {
					h.sendMessage(chatID, "Ошибка: "+err.Error(), "empty")
				} else {
					fileBytes := tgbotapi.FileBytes{Name: "teams.csv", Bytes: csvData}
					h.bot.Send(tgbotapi.NewDocument(chatID, fileBytes))
				}
				continue
			}

			if strings.HasPrefix(text, "/broadcast ") {
				msgText := strings.TrimPrefix(text, "/broadcast ")
				ids, _ := h.useCase.GetBroadcastList()
				for _, id := range ids {
					h.sendMessage(id, "СООБЩЕНИЕ ОТ ОРГАНИЗАТОРОВ:\n\n"+msgText, "empty")
				}
				h.sendMessage(chatID, fmt.Sprintf("Рассылка на %d чел. завершена.", len(ids)), "empty")
				continue
			}

			if text == "/close_reg" {
				h.useCase.SetRegistrationOpen(false)
				h.sendMessage(chatID, "Регистрация закрыта.", "empty")
				continue
			}
			if text == "/open_reg" {
				h.useCase.SetRegistrationOpen(true)
				h.sendMessage(chatID, "Регистрация открыта.", "empty")
				continue
			}

			if strings.HasPrefix(text, "/del_team ") {
				name := strings.TrimPrefix(text, "/del_team ")
				h.sendMessage(chatID, h.useCase.AdminDeleteTeam(name), "empty")
				continue
			}

			if strings.HasPrefix(text, "/reset_user ") {
				idStr := strings.TrimPrefix(text, "/reset_user ")
				id, _ := strconv.ParseInt(idStr, 10, 64)
				h.sendMessage(chatID, h.useCase.AdminResetUser(id), "empty")
				continue
			}
		}

		if strings.HasPrefix(text, "/edit_player") {
			parts := strings.Fields(text)
			if len(parts) != 2 {
				response = "Используйте: /edit_player [номер]"
			} else {
				slot, _ := strconv.Atoi(parts[1])
				response, kbType = h.useCase.StartEditPlayer(chatID, slot)
			}
			h.sendMessage(chatID, response, kbType)
			continue
		}

		switch text {
		case "/start":
			response = "Valhalla Cup Bot\n\n" +
				"/reg_solo - Регистрация (соло)\n" +
				"/reg_team - Регистрация (команда)\n" +
				"/my_team - Мой состав\n" +
				"/edit_player [№] - Изменить данные игрока\n" +
				"/checkin - Подтвердить участие\n" +
				"/report - Отправить результат матча\n" +
				"/delete_team - Удалить команду"
			kbType = "empty"

		case "/reg_solo":
			response, kbType = h.useCase.StartSoloRegistration(chatID)
		case "/reg_team":
			response, kbType = h.useCase.StartTeamRegistration(chatID)
		case "/my_team":
			response = h.useCase.GetTeamInfo(chatID)
			kbType = "empty"
		case "/checkin":
			response = h.useCase.ToggleCheckIn(chatID)
			kbType = "empty"
		case "/delete_team":
			response = h.useCase.DeleteTeam(chatID)
			kbType = "empty"
		case "/report":
			response, kbType = h.useCase.StartReport(chatID)

		default:
			response, kbType = h.useCase.HandleUserInput(chatID, text)
		}

		h.sendMessage(chatID, response, kbType)
	}
}

func (h *TelegramHandler) sendMessage(chatID int64, text string, kbType string) {
	if text == "" {
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)

	switch kbType {
	case "role":
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
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
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Отмена"),
			),
		)
	case "cancel":
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Отмена"),
			),
		)
	default:
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	}

	h.bot.Send(msg)
}
