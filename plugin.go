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

var messageRequestMap = make(map[string]int)

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
		fmt.Println("Watcher ready, syncing existing requests")
		err := a.syncExisting(ctx)
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
		msgID, ok := messageRequestMap[r.GetName()]
		if !ok {
			return nil
		}
		a.updateExisting(ctx, r, msgID)
		return nil
	}
	a.postRequest(ctx, r)
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

	action, reqID, ok := strings.Cut(cb.Data, ":")
	if !ok {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "Failed to parse action",
			ShowAlert:       false,
		})
		return
	}
	switch action {
	case approveButtonID:
		proposedState = types.RequestState_APPROVED
	case denyButtonID:
		proposedState = types.RequestState_DENIED
	default:
		fmt.Println("Skipping unknown action", action)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "Unknown action",
			ShowAlert:       false,
		})
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
		fmt.Printf("Error submitting access review: %v\n", err)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "Failed to submit the review: " + trace.UserMessage(err),
			ShowAlert:       true,
		})
		return
	}
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
		Text:            "Decision recorded",
		ShowAlert:       false,
	})
}

func (a *AccessRequestPlugin) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	fmt.Printf("Nothing to do \n")
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

func (a *AccessRequestPlugin) syncExisting(ctx context.Context) error {

	var pendingRqs = make(map[string]bool)

	reqs, err := a.TeleportClient.GetAccessRequests(ctx, types.AccessRequestFilter{
		State: types.RequestState_PENDING,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, r := range reqs {
		pendingRqs[r.GetName()] = true
		a.postRequest(ctx, r)

	}
	for reqID, msgID := range messageRequestMap {
		if pendingRqs[reqID] {
			continue //skip updates for known pending messages
		}
		r, err := a.TeleportClient.GetAccessRequests(ctx, types.AccessRequestFilter{
			ID: reqID,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if len(r) == 0 {
			fmt.Printf("request with ID %s has expired or has been deleted\n", reqID)
			if _, err := a.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    a.cfg.TgChatId,
				MessageID: msgID,
				Text:      "Access Request with ID " + reqID + " has expired or has been deleted\n",
			}); err != nil {
				fmt.Printf("editing the message failed:\n%v\n", err)
				continue
			}
			delete(messageRequestMap, reqID)
			continue
		}

		a.updateExisting(ctx, r[0], msgID)
	}
	return nil
}

func (a *AccessRequestPlugin) postRequest(ctx context.Context, r types.AccessRequest) {

	if _, ok := messageRequestMap[r.GetName()]; ok {
		return
	}
	msg, err := a.tb.sendKeyboard(ctx, a.bot, r)
	if err != nil {
		fmt.Printf("failed to send message, skipping adding msgID to the map\n")
		return
	}
	msgID := msg.ID
	messageRequestMap[r.GetName()] = msgID
}

func (a *AccessRequestPlugin) updateExisting(ctx context.Context, r types.AccessRequest, msgID int) {

	state := r.GetState()
	var status string
	var reviewers []string

	rqText := a.tb.formatRequest(r)

	switch state {
	case types.RequestState_PENDING, types.RequestState_NONE:
		return
	case types.RequestState_APPROVED:
		status = "Approved"
	case types.RequestState_DENIED:
		status = "Denied"
	case types.RequestState_PROMOTED:
		status = "Promoted to Access List"
	default:
		fmt.Printf("Unknown request state\n")
		return
	}
	for _, review := range r.GetReviews() {
		if review.ProposedState == state {
			reviewers = append(reviewers, review.Author)
		}
	}
	suffix := ""
	if len(reviewers) > 0 { //may be empty for tctl approvals
		suffix = " by " + strings.Join(reviewers, ", ")
	}
	if _, err := a.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    a.cfg.TgChatId,
		MessageID: msgID,
		Text:      "Access Request:\n" + rqText + "\nThis request has been resolved.\nStatus: " + status + suffix,
	}); err != nil {
		fmt.Printf("editing the message failed:\n%v\n", err)
		return
	}
	delete(messageRequestMap, r.GetName())
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
