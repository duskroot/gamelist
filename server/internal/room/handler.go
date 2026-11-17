package room

import (
	"context"
	"encoding/json"
	"errors"

	"den/internal/envelope"
	"den/internal/realtime"

	"github.com/cloudwego/hertz/pkg/app"
)

type Handler struct {
	Store     *Store
	Publisher realtime.Publisher
}

type createReq struct {
	Mode     string `json:"Mode"`
	MaxSeats int    `json:"MaxSeats"`
}

type codeReq struct {
	RoomCode string `json:"RoomCode"`
}

type roomReq struct {
	RoomId string `json:"RoomId"`
	Ready  bool   `json:"Ready"`
}

func (h *Handler) CreateRoom(ctx context.Context, c *app.RequestContext) {
	const action = "CreateRoom"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req createReq
	if !decode(c, action, &req) {
		return
	}
	roomID, roomCode, err := h.Store.Create(ctx, userID, req.Mode, req.MaxSeats)
	if err != nil {
		fail(c, action, err)
		return
	}
	h.emit(ctx, roomID)
	envelope.OK(c, "room", action, map[string]any{"RoomId": roomID, "RoomCode": roomCode, "SeatNo": 0})
}

func (h *Handler) JoinRoom(ctx context.Context, c *app.RequestContext) {
	const action = "JoinRoom"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req codeReq
	if !decode(c, action, &req) {
		return
	}
	roomID, seatNo, err := h.Store.Join(ctx, userID, req.RoomCode)
	if err != nil {
		fail(c, action, err)
		return
	}
	h.emit(ctx, roomID)
	envelope.OK(c, "room", action, map[string]any{"RoomId": roomID, "SeatNo": seatNo})
}

func (h *Handler) LeaveRoom(ctx context.Context, c *app.RequestContext) {
	const action = "LeaveRoom"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req roomReq
	if !decode(c, action, &req) {
		return
	}
	if err := h.Store.Leave(ctx, userID, req.RoomId); err != nil {
		fail(c, action, err)
		return
	}
	h.emit(ctx, req.RoomId)
	envelope.OK(c, "room", action, map[string]string{"RoomId": req.RoomId})
}

func (h *Handler) Ready(ctx context.Context, c *app.RequestContext) {
	const action = "Ready"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req roomReq
	if !decode(c, action, &req) {
		return
	}
	if err := h.Store.Ready(ctx, userID, req.RoomId, req.Ready); err != nil {
		fail(c, action, err)
		return
	}
	h.emit(ctx, req.RoomId)
	envelope.OK(c, "room", action, map[string]any{"RoomId": req.RoomId, "Ready": req.Ready})
}

func (h *Handler) DescribeRoom(ctx context.Context, c *app.RequestContext) {
	const action = "DescribeRoom"
	if _, ok := userID(c); !ok {
		unauthorized(c, action)
		return
	}
	var req roomReq
	if !decode(c, action, &req) {
		return
	}
	view, err := h.Store.Describe(ctx, req.RoomId)
	if err != nil {
		fail(c, action, err)
		return
	}
	envelope.OK(c, "room", action, view)
}

func (h *Handler) StartGame(ctx context.Context, c *app.RequestContext) {
	const action = "StartGame"
	userID, ok := userID(c)
	if !ok {
		unauthorized(c, action)
		return
	}
	var req roomReq
	if !decode(c, action, &req) {
		return
	}
	roundID, err := h.Store.Start(ctx, userID, req.RoomId)
	if err != nil {
		fail(c, action, err)
		return
	}
	h.emit(ctx, req.RoomId)
	envelope.OK(c, "room", action, map[string]string{"RoundId": roundID})
}

func (h *Handler) emit(ctx context.Context, roomID string) {
	if h.Publisher == nil {
		return
	}
	view, err := h.Store.Describe(ctx, roomID)
	if err != nil {
		return
	}
	h.Publisher.EmitRoom(roomID, realtime.Event{Service: "room", Action: "RoomChanged", Result: view})
}

func decode(c *app.RequestContext, action string, dest any) bool {
	raw, err := c.Body()
	if err != nil || json.Unmarshal(raw, dest) != nil {
		envelope.Fail(c, 200, "room", action, "100002", "Invalid Parameter", "body must be JSON")
		return false
	}
	return true
}

func unauthorized(c *app.RequestContext, action string) {
	envelope.Fail(c, 200, "room", action, "100003", "Unauthorized", "")
}

func fail(c *app.RequestContext, action string, err error) {
	switch {
	case errors.Is(err, ErrBadInput):
		envelope.Fail(c, 200, "room", action, "100002", "Invalid Parameter", "")
	case errors.Is(err, ErrNotFound):
		envelope.Fail(c, 200, "room", action, "200001", "Room Not Found", "")
	case errors.Is(err, ErrFull):
		envelope.Fail(c, 200, "room", action, "200002", "Room Full", "")
	case errors.Is(err, ErrNotIn):
		envelope.Fail(c, 200, "room", action, "200003", "Not In Room", "")
	case errors.Is(err, ErrNotHost):
		envelope.Fail(c, 200, "room", action, "200004", "Not Host", "")
	case errors.Is(err, ErrNotReady):
		envelope.Fail(c, 200, "room", action, "200005", "Not Ready", "")
	case errors.Is(err, ErrIllegal):
		envelope.Fail(c, 200, "room", action, "300002", "Illegal Action", "")
	default:
		envelope.Fail(c, 200, "room", action, "100001", "System Error", "")
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
