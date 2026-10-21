package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gravitational/teleport/api/client"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

const (
	approveButtonID = "request_approve"
	denyButtonID    = "request_deny"
)

type AccessRequestPlugin struct {
	cfg            Config
	tb             *TgBot
	bot            *bot.Bot
	TeleportClient *client.Client
}

func (a *AccessRequestPlugin) HandleEvent(ctx context.Context, e types.Event) error {
	if e.Resource == nil {
		return nil
	}

	if _, ok := e.Resource.(*types.WatchStatusV1); ok {
		fmt.Println("Watcher ready, syncing pending requests")
		err := a.syncPending(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	r, ok := e.Resource.(types.AccessRequest)
	if !ok {
		fmt.Printf("Unknown (%T) event received, skipping.\n", e.Resource)
		return nil
	}
	if e.Type != types.OpPut || !r.GetState().IsPending() {
		return nil
	}
	a.tb.sendKeyboard(ctx, a.bot, r)
	return nil
}

func (a *AccessRequestPlugin) handleDecision(ctx context.Context, b *bot.Bot, update *models.Update) {

	var proposedState types.RequestState
	cb := update.CallbackQuery

	if cb.Message.Message == nil {
		return // ignore updates that aren't plain messages
	}
	username, err := a.checkUserTgTrait(ctx, cb.From.ID)
	if err != nil {
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
	action, reqID, ok := strings.Cut(cb.Data, ":")
	if !ok {
		return
	}
	switch action {
	case approveButtonID:
		proposedState = types.RequestState_APPROVED
	case denyButtonID:
		proposedState = types.RequestState_DENIED
	default:
		fmt.Println("Skipping unknown aciton", action)
		return
	}
	if _, err := a.TeleportClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: reqID,
		Review: types.AccessReview{
			Author:                    username,
			SubmittedOnBehalfOfAuthor: true,
			ProposedState:             proposedState,
			Created:                   time.Now(),
		},
	}); err != nil {
		fmt.Printf("Error submitting access review: %v", err)

		if strings.Contains(err.Error(), "the access request has been already") {
			// Convert error to NotFound to simplify user-facing error replies.
			fmt.Println("request has already been resolved")
			return
		}
		return
	}
	//TODO deactivate chat buttons after decision
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    cb.Message.Message.Chat.ID,
		MessageID: cb.Message.Message.ID,
		Text:      cb.Message.Message.Text + "\n\nThis request has been reviewed by " + username + "\nDecision: " + action,
	})

	// b.SendMessage(ctx, &bot.SendMessageParams{
	// 	ChatID: cb.Message.Message.Chat.ID,
	// 	Text:   "Selected action: " + cb.Data,
	// })
}

func (a *AccessRequestPlugin) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	fmt.Printf("Nothing to do ")
}

func (a *AccessRequestPlugin) checkUserTgTrait(ctx context.Context, tgUID int64) (string, error) {
	s := strconv.Itoa(int(tgUID))
	usr, err := findUserByTrait(ctx, a.TeleportClient, a.cfg.TgTraitKey, s)
	if err != nil {
		return "", err
	}
	return usr, nil

}

func (a *AccessRequestPlugin) Run(ctx context.Context) error {
	for {
		err := a.watchEvents(ctx)
		if ctx.Err() != nil {
			return nil // shutting down
		}
		fmt.Printf("watcher stopped: %v, reconnecting in 5s\n", err)
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return nil
		}
	}
}

func (a *AccessRequestPlugin) watchEvents(ctx context.Context) error {

	watch, err := a.TeleportClient.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			types.WatchKind{Kind: types.KindAccessRequest},
		},
	})

	if err != nil {
		return trace.Wrap(err)
	}
	defer watch.Close()

	fmt.Println("Starting the watcher job")

	for {
		select {
		case e := <-watch.Events():
			if err := a.HandleEvent(ctx, e); err != nil {
				return trace.Wrap(err)
			}
		case <-watch.Done():
			return trace.Wrap(watch.Error(), "watcher closed")
		}
	}
}

func (a *AccessRequestPlugin) syncPending(ctx context.Context) error {
	reqs, err := a.TeleportClient.GetAccessRequests(ctx, types.AccessRequestFilter{
		State: types.RequestState_PENDING,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	//TODO: add dedupe for sent messages
	for _, r := range reqs {
		a.postRequest(ctx, r)
	}
	return nil
}
func (a *AccessRequestPlugin) postRequest(ctx context.Context, r types.AccessRequest) {
	a.tb.sendKeyboard(ctx, a.bot, r)
}

func findUserByTrait(ctx context.Context, clt *client.Client, key, value string) (string, error) {
	var token string
	for {
		resp, err := clt.ListUsers(ctx, &userspb.ListUsersRequest{
			PageSize:  100,
			PageToken: token,
		})
		if err != nil {
			return "", err
		}
		for _, u := range resp.Users {
			for _, v := range u.GetTraits()[key] {
				if v == value {
					return u.GetName(), nil
				}
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		token = resp.NextPageToken
	}
	return "", fmt.Errorf("no user with trait %s=%s", key, value)

}
