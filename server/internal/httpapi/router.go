package httpapi

import (
	"den/internal/account"
	"den/internal/dice"
	"den/internal/envelope"
	"den/internal/hall"
	"den/internal/realtime"
	"den/internal/room"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Pool      *pgxpool.Pool
	Redis     *redis.Client
	Publisher realtime.Publisher
	Roller    dice.Roller
}

func New(deps Deps) *server.Hertz {
	if deps.Publisher == nil {
		deps.Publisher = realtime.Nop{}
	}
	roller := deps.Roller
	if roller == nil {
		roller = dice.CryptoRoller
	}
	h := server.New()
	h.Use(envelope.Recovery())
	acc := &account.Handler{Pool: deps.Pool, Redis: deps.Redis}
	h.POST("/den/v1/account/GuestLogin", acc.GuestLogin)
	h.POST("/den/v1/account/UpdateNickname", Authenticate(deps.Redis, "account", "UpdateNickname"), acc.UpdateNickname)
	hallHandler := &hall.Handler{Pool: deps.Pool, Redis: deps.Redis}
	h.POST("/den/v1/hall/EnterHall", Authenticate(deps.Redis, "hall", "EnterHall"), hallHandler.EnterHall)
	rooms := &room.Handler{Store: &room.Store{Pool: deps.Pool}, Publisher: deps.Publisher}
	h.POST("/den/v1/room/CreateRoom", Authenticate(deps.Redis, "room", "CreateRoom"), rooms.CreateRoom)
	h.POST("/den/v1/room/JoinRoom", Authenticate(deps.Redis, "room", "JoinRoom"), rooms.JoinRoom)
	h.POST("/den/v1/room/LeaveRoom", Authenticate(deps.Redis, "room", "LeaveRoom"), rooms.LeaveRoom)
	h.POST("/den/v1/room/Ready", Authenticate(deps.Redis, "room", "Ready"), rooms.Ready)
	h.POST("/den/v1/room/DescribeRoom", Authenticate(deps.Redis, "room", "DescribeRoom"), rooms.DescribeRoom)
	h.POST("/den/v1/room/StartGame", Authenticate(deps.Redis, "room", "StartGame"), rooms.StartGame)
	diceHandler := &dice.Handler{
		Service:   &dice.Service{Pool: deps.Pool, Roller: roller},
		Publisher: deps.Publisher,
	}
	h.POST("/den/v1/dice/Roll", Authenticate(deps.Redis, "dice", "Roll"), diceHandler.Roll)
	h.POST("/den/v1/dice/Call", Authenticate(deps.Redis, "dice", "Call"), diceHandler.Call)
	h.POST("/den/v1/dice/Open", Authenticate(deps.Redis, "dice", "Open"), diceHandler.Open)
	h.POST("/den/v1/dice/DescribeRound", Authenticate(deps.Redis, "dice", "DescribeRound"), diceHandler.DescribeRound)
	return h
}
