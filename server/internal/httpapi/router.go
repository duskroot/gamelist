package httpapi

import (
	"den/internal/account"
	"den/internal/realtime"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Pool      *pgxpool.Pool
	Redis     *redis.Client
	Publisher realtime.Publisher
}

func New(deps Deps) *server.Hertz {
	if deps.Publisher == nil {
		deps.Publisher = realtime.Nop{}
	}
	h := server.New()
	acc := &account.Handler{Pool: deps.Pool, Redis: deps.Redis}
	h.POST("/den/v1/account/GuestLogin", acc.GuestLogin)
	return h
}
