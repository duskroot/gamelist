package match

import (
	"context"
	"encoding/json"
	"errors"

	"den/internal/envelope"

	"github.com/cloudwego/hertz/pkg/app"
)

type Handler struct {
	Service *Service
}

type startReq struct {
	Mode     string `json:"Mode"`
	MaxSeats int    `json:"MaxSeats"`
}

type matchReq struct {
	MatchId string `json:"MatchId"`
}

func (h *Handler) StartMatch(ctx context.Context, c *app.RequestContext) {
	const action = "StartMatch"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req startReq
	if !decode(c, action, &req) {
		return
	}
	matchID, status, err := h.Service.Start(ctx, userID, req.Mode, req.MaxSeats)
	if err != nil {
		fail(c, action, err)
		return
	}
	envelope.OK(c, "match", action, map[string]string{"MatchId": matchID, "Status": status})
}

func (h *Handler) DescribeMatch(ctx context.Context, c *app.RequestContext) {
	const action = "DescribeMatch"
	if _, ok := userID(c); !ok {
		unauthorized(c, action)
		return
	}
	var req matchReq
	if !decode(c, action, &req) {
		return
	}
	view, err := h.Service.Describe(ctx, req.MatchId)
	if err != nil {
		fail(c, action, err)
		return
	}
	envelope.OK(c, "match", action, view)
}

func (h *Handler) CancelMatch(ctx context.Context, c *app.RequestContext) {
	const action = "CancelMatch"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req matchReq
	if !decode(c, action, &req) {
		return
	}
	if err := h.Service.Cancel(ctx, userID, req.MatchId); err != nil {
		fail(c, action, err)
		return
	}
	envelope.OK(c, "match", action, map[string]string{"MatchId": req.MatchId, "Status": "Canceled"})
}

func decode(c *app.RequestContext, action string, dest any) bool {
	raw, err := c.Body()
	if err != nil || json.Unmarshal(raw, dest) != nil {
		envelope.Fail(c, 200, "match", action, "100002", "Invalid Parameter", "body must be JSON")
		return false
	}
	return true
}

func unauthorized(c *app.RequestContext, action string) {
	envelope.Fail(c, 200, "match", action, "100003", "Unauthorized", "")
}

func fail(c *app.RequestContext, action string, err error) {
	switch {
	case errors.Is(err, ErrBadInput):
		envelope.Fail(c, 200, "match", action, "100002", "Invalid Parameter", "")
	case errors.Is(err, ErrNotFound):
		envelope.Fail(c, 200, "match", action, "400001", "Match Not Found", "")
	case errors.Is(err, ErrClosed):
		envelope.Fail(c, 200, "match", action, "400002", "Match Closed", "")
	case errors.Is(err, ErrAlready):
		envelope.Fail(c, 200, "match", action, "400003", "Already Matching", "")
	default:
		envelope.Fail(c, 200, "match", action, "100001", "System Error", "")
	}
}

func userID(c *app.RequestContext) (string, bool) {
	v, ok := c.Get("UserID")
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok && id != ""
}
