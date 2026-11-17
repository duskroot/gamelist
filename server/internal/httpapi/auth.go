package httpapi

import (
	"context"
	"strings"

	"den/internal/envelope"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/redis/go-redis/v9"
)

func Authenticate(rdb *redis.Client, service, action string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		header := string(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(header, "Bearer ")
		if token == "" || token == header {
			envelope.Fail(c, 200, service, action, "100003", "Unauthorized", "")
			c.Abort()
			return
		}
		userID, err := rdb.Get(ctx, "den:token:"+token).Result()
		if err != nil || userID == "" {
			envelope.Fail(c, 200, service, action, "100003", "Unauthorized", "")
			c.Abort()
			return
		}
		c.Set("UserID", userID)
		c.Next(ctx)
	}
}

func UserID(c *app.RequestContext) (string, bool) {
	v, ok := c.Get("UserID")
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok && id != ""
}
