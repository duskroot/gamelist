package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"den/internal/envelope"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	Pool  *pgxpool.Pool
	Redis *redis.Client
}

type guestReq struct {
	DeviceId string `json:"DeviceId"`
	Nickname string `json:"Nickname"`
}

func (h *Handler) GuestLogin(ctx context.Context, c *app.RequestContext) {
	var req guestReq
	raw, err := c.Body()
	if err != nil {
		envelope.Fail(c, 200, "account", "GuestLogin", "100002", "Invalid Parameter", "body must be JSON")
		return
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		envelope.Fail(c, 200, "account", "GuestLogin", "100002", "Invalid Parameter", "body must be JSON")
		return
	}
	if _, err := uuid.Parse(req.DeviceId); err != nil {
		envelope.Fail(c, 200, "account", "GuestLogin", "100002", "Invalid Parameter", "DeviceId must be a UUID")
		return
	}
	if req.Nickname != "" && (utf8.RuneCountInString(req.Nickname) < 2 || utf8.RuneCountInString(req.Nickname) > 12) {
		envelope.Fail(c, 200, "account", "GuestLogin", "100002", "Invalid Parameter", "Nickname length must be 2 to 12")
		return
	}
	userID := uuid.NewString()
	nickname := req.Nickname
	if nickname == "" {
		nickname = defaultNickname()
	}
	err = h.Pool.QueryRow(ctx, `SELECT user_id, nickname FROM users WHERE device_id=$1`, req.DeviceId).Scan(&userID, &nickname)
	if err == pgx.ErrNoRows {
		if req.Nickname == "" {
			nickname = defaultNickname()
		}
		_, err = h.Pool.Exec(ctx, `INSERT INTO users(user_id, device_id, nickname) VALUES ($1,$2,$3)`, userID, req.DeviceId, nickname)
	}
	if err != nil {
		envelope.Fail(c, 200, "account", "GuestLogin", "100001", "System Error", "")
		return
	}
	if old, err := h.Redis.Get(ctx, "den:user-token:"+userID).Result(); err == nil && old != "" {
		_ = h.Redis.Del(ctx, "den:token:"+old).Err()
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		envelope.Fail(c, 200, "account", "GuestLogin", "100001", "System Error", "")
		return
	}
	token := hex.EncodeToString(buf)
	if err := h.Redis.Set(ctx, "den:token:"+token, userID, 7*24*time.Hour).Err(); err != nil {
		envelope.Fail(c, 200, "account", "GuestLogin", "100001", "System Error", "")
		return
	}
	if err := h.Redis.Set(ctx, "den:user-token:"+userID, token, 7*24*time.Hour).Err(); err != nil {
		envelope.Fail(c, 200, "account", "GuestLogin", "100001", "System Error", "")
		return
	}
	envelope.OK(c, "account", "GuestLogin", map[string]string{"UserId": userID, "Token": token, "Nickname": nickname})
}

func defaultNickname() string {
	var n [2]byte
	_, _ = rand.Read(n[:])
	return fmt.Sprintf("玩家%04d", (int(n[0])<<8|int(n[1]))%10000)
}
