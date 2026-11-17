package dice

import (
	"context"
	"encoding/json"
	"errors"

	"den/internal/envelope"
	"den/internal/realtime"

	"github.com/cloudwego/hertz/pkg/app"
)

type Handler struct {
	Service   *Service
	Publisher realtime.Publisher
}

type roundReq struct {
	RoundId string `json:"RoundId"`
}

type callReq struct {
	RoundId string `json:"RoundId"`
	Count   int    `json:"Count"`
	Face    int    `json:"Face"`
}

func (h *Handler) Roll(ctx context.Context, c *app.RequestContext) {
	const action = "Roll"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req roundReq
	if !decode(c, action, &req) {
		return
	}
	res, err := h.Service.Roll(ctx, userID, req.RoundId)
	if err != nil {
		fail(c, action, err)
		return
	}
	h.emitUser(res.RoomID, res.UserID, "DiceRolled", map[string]any{
		"RoundId": res.RoundID,
		"SeatNo":  res.SeatNo,
		"Dice":    res.Dice,
	})
	h.emitRoom(res.RoomID, "DiceHidden", map[string]any{
		"RoundId": res.RoundID,
		"SeatNo":  res.SeatNo,
	})
	if res.Resolved {
		h.emitRoom(res.RoomID, "RoundResolved", map[string]any{
			"RoundId":    res.RoundID,
			"Outcome":    res.Outcome,
			"WinnerSeat": res.WinnerSeat,
		})
	}
	envelope.OK(c, "dice", action, map[string]any{
		"RoundId": res.RoundID,
		"SeatNo":  res.SeatNo,
		"Dice":    res.Dice,
	})
}

func (h *Handler) Call(ctx context.Context, c *app.RequestContext) {
	const action = "Call"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req callReq
	if !decode(c, action, &req) {
		return
	}
	res, err := h.Service.Call(ctx, userID, req.RoundId, req.Count, req.Face)
	if err != nil {
		fail(c, action, err)
		return
	}
	h.emitRoom(res.RoomID, "DiceCalled", map[string]any{
		"RoundId": res.RoundID,
		"SeatNo":  res.SeatNo,
		"Count":   res.Count,
		"Face":    res.Face,
	})
	envelope.OK(c, "dice", action, map[string]any{
		"RoundId": res.RoundID,
		"SeatNo":  res.SeatNo,
		"Count":   res.Count,
		"Face":    res.Face,
	})
}

func (h *Handler) Open(ctx context.Context, c *app.RequestContext) {
	const action = "Open"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req roundReq
	if !decode(c, action, &req) {
		return
	}
	res, err := h.Service.Open(ctx, userID, req.RoundId)
	if err != nil {
		fail(c, action, err)
		return
	}
	h.emitRoom(res.RoomID, "DiceOpened", map[string]any{
		"RoundId": res.RoundID,
		"Hands":   res.Hands,
	})
	h.emitRoom(res.RoomID, "RoundResolved", map[string]any{
		"RoundId":    res.RoundID,
		"Outcome":    res.Outcome,
		"WinnerSeat": res.WinnerSeat,
	})
	envelope.OK(c, "dice", action, map[string]any{
		"RoundId":    res.RoundID,
		"Outcome":    res.Outcome,
		"WinnerSeat": res.WinnerSeat,
	})
}

func (h *Handler) DescribeRound(ctx context.Context, c *app.RequestContext) {
	const action = "DescribeRound"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req roundReq
	if !decode(c, action, &req) {
		return
	}
	view, err := h.Service.Describe(ctx, userID, req.RoundId)
	if err != nil {
		fail(c, action, err)
		return
	}
	envelope.OK(c, "dice", action, view)
}

func (h *Handler) emitRoom(roomID, action string, result any) {
	if h.Publisher == nil {
		return
	}
	h.Publisher.EmitRoom(roomID, realtime.Event{Service: "dice", Action: action, Result: result})
}

func (h *Handler) emitUser(roomID, userID, action string, result any) {
	if h.Publisher == nil {
		return
	}
	h.Publisher.EmitUser(roomID, userID, realtime.Event{Service: "dice", Action: action, Result: result})
}

func decode(c *app.RequestContext, action string, dest any) bool {
	raw, err := c.Body()
	if err != nil || json.Unmarshal(raw, dest) != nil {
		envelope.Fail(c, 200, "dice", action, "100002", "Invalid Parameter", "body must be JSON")
		return false
	}
	return true
}

func unauthorized(c *app.RequestContext, action string) {
	envelope.Fail(c, 200, "dice", action, "100003", "Unauthorized", "")
}

func fail(c *app.RequestContext, action string, err error) {
	switch {
	case errors.Is(err, ErrBadInput):
		envelope.Fail(c, 200, "dice", action, "100002", "Invalid Parameter", "")
	case errors.Is(err, ErrNotFound):
		envelope.Fail(c, 200, "dice", action, "200001", "Room Not Found", "")
	case errors.Is(err, ErrNotIn):
		envelope.Fail(c, 200, "dice", action, "200003", "Not In Room", "")
	case errors.Is(err, ErrNotTurn):
		envelope.Fail(c, 200, "dice", action, "300001", "Not Your Turn", "")
	case errors.Is(err, ErrIllegal):
		envelope.Fail(c, 200, "dice", action, "300002", "Illegal Action", "")
	default:
		envelope.Fail(c, 200, "dice", action, "100001", "System Error", "")
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
