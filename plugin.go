package main

import (
	 "context"
	 "fmt"
	 "github.com/go-telegram/bot"
	 "github.com/go-telegram/bot/models"
)

//tmp plumbing to replace actual provider
var approvers = map[int64]bool{
	111111111: true,
	222222222: true,
    110901040: true,

}
var requestID = "01a154cd-9af0-71fd-8acf-e3d35ce28ce1"

type AccessRequestPlugin struct{
    
}

func (a *AccessRequestPlugin) handleDecision(ctx context.Context, b *bot.Bot, update *models.Update){
	cb := update.CallbackQuery
	if cb.Message.Message == nil {
		return // ignore updates that aren't plain messages
	}
	if ! a.checkUserTgTrait(cb.From.ID) {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "You're not allowed to do this.",
			ShowAlert:       true, // shows a popup only the clicker sees
		})
		fmt.Println("Rejected the decision from Telegram user with ID", cb.From.ID)
		return
	}
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
		ShowAlert:       false,
	})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: cb.Message.Message.Chat.ID,
		Text:   "Selected action: " + cb.Data,
	})	
}

func (a *AccessRequestPlugin) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	fmt.Printf("Nothing to do ")
}

func (a *AccessRequestPlugin) checkUserTgTrait (tgUID int64) bool{
	if !approvers[tgUID]{
		return false
	}
	return true
}