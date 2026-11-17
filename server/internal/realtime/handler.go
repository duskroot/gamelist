package realtime

import (
	"context"
	"encoding/json"
	"log"

	"den/internal/envelope"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"github.com/hertz-contrib/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.HertzUpgrader{
	CheckOrigin: func(*app.RequestContext) bool { return true },
}

type Handler struct {
	Hub   *Hub
	Redis *redis.Client
	Pool  *pgxpool.Pool
}

type subscribeReq struct {
	Token  string `json:"Token"`
	RoomId string `json:"RoomId"`
}

type errorBody struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type failure struct {
	ResponseMetadata envelope.Metadata `json:"ResponseMetadata"`
	Error            errorBody         `json:"Error"`
}

func (h *Handler) Subscribe(_ context.Context, c *app.RequestContext) {
	_ = upgrader.Upgrade(c, func(conn *websocket.Conn) {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			_ = conn.Close()
			return
		}
		var req subscribeReq
		if json.Unmarshal(payload, &req) != nil || req.Token == "" {
			failConn(conn, "100003", "Unauthorized")
			return
		}
		userID, err := h.Redis.Get(context.Background(), "den:token:"+req.Token).Result()
		if err != nil || userID == "" {
			failConn(conn, "100003", "Unauthorized")
			return
		}
		var seated bool
		err = h.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM room_seats WHERE room_id=$1 AND user_id=$2)`, req.RoomId, userID).Scan(&seated)
		if err != nil {
			failConn(conn, "100001", "System Error")
			return
		}
		if !seated {
			failConn(conn, "200003", "Not In Room")
			return
		}
		h.Hub.add(req.RoomId, userID, conn)
		defer h.Hub.remove(req.RoomId, userID, conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
}

func failConn(conn *websocket.Conn, code, message string) {
	meta := envelope.Metadata{
		RequestId: uuid.NewString(),
		Action:    "Subscribe",
		Version:   "v1",
		Service:   "room",
	}
	log.Printf("RequestId=%s Action=%s Version=%s Service=%s", meta.RequestId, meta.Action, meta.Version, meta.Service)
	_ = conn.WriteJSON(failure{ResponseMetadata: meta, Error: errorBody{Code: code, Message: message}})
	_ = conn.Close()
}
