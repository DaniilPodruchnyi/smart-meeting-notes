package telegram

import "gopkg.in/telebot.v3"

type telegramSender struct {
	bot *telebot.Bot
}

func NewTelegramSender(bot *telebot.Bot) *telegramSender {
	return &telegramSender{bot: bot}
}

func (t *telegramSender) SendToUser(chatID int64, text string) error {
	_, err := t.bot.Send(&telebot.Chat{ID: chatID}, text)
	return err
}
