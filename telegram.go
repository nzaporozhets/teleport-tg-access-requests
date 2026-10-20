package main

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type TgBot struct {
	chatID int64
}

func (tb *TgBot) sendKeyboard(ctx context.Context, b *bot.Bot, rqID string) {
	rqText := fmt.Sprintf("New access request: %s", rqID)
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Approve", CallbackData: approveButtonID + ":" + rqID},
				{Text: "Deny", CallbackData: denyButtonID + ":" + rqID},
			},
		},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      tb.chatID,
		Text:        rqText,
		ReplyMarkup: kb,
	})
	if err != nil {
		// handle/log properly in real code
		println("send failed:", err.Error())
	}
}
