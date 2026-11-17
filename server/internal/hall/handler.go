package hall

import (
	"context"
	"time"

	"den/internal/envelope"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	Pool  *pgxpool.Pool
	Redis *redis.Client
}

func (h *Handler) EnterHall(ctx context.Context, c *app.RequestContext) {
	userID, ok := userID(c)
	if !ok {
		envelope.Fail(c, 200, "hall", "EnterHall", "100003", "Unauthorized", "")
		return
	}
	var nickname string
	if err := h.Pool.QueryRow(ctx, `SELECT nickname FROM users WHERE user_id=$1`, userID).Scan(&nickname); err != nil {
		envelope.Fail(c, 200, "hall", "EnterHall", "100001", "System Error", "")
		return
	}
	if err := h.Redis.Set(ctx, "den:online:"+userID, "1", 60*time.Second).Err(); err != nil {
		envelope.Fail(c, 200, "hall", "EnterHall", "100001", "System Error", "")
		return
	}
	envelope.OK(c, "hall", "EnterHall", map[string]string{"UserId": userID, "Nickname": nickname})
}

func userID(c *app.RequestContext) (string, bool) {
	v, ok := c.Get("UserID")
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok && id != ""
}
