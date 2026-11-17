package httpapi

import (
	"den/internal/account"
	"den/internal/dice"
	"den/internal/envelope"
	"den/internal/hall"
	"den/internal/match"
	"den/internal/realtime"
	"den/internal/room"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Pool      *pgxpool.Pool
	Redis     *redis.Client
	Publisher realtime.Publisher
	Roller    dice.Roller
	Hub       *realtime.Hub
	Addr      string
}

func New(deps Deps) *server.Hertz {
	opts := []config.Option{server.WithTransport(standard.NewTransporter)}
	if deps.Addr != "" {
		opts = append(opts, server.WithHostPorts(deps.Addr))
	}
	h := server.New(opts...)
	h.NoHijackConnPool = true
	Register(h, deps)
	return h
}

func Register(h *server.Hertz, deps Deps) {
	if deps.Hub != nil {
		deps.Publisher = deps.Hub
	} else if deps.Publisher == nil {
		deps.Publisher = realtime.Nop{}
	}
	roller := deps.Roller
	if roller == nil {
		roller = dice.CryptoRoller
	}
	h.Use(envelope.Recovery())
	acc := &account.Handler{Pool: deps.Pool, Redis: deps.Redis}
	h.POST("/den/v1/account/GuestLogin", acc.GuestLogin)
	h.POST("/den/v1/account/UpdateNickname", Authenticate(deps.Redis, "account", "UpdateNickname"), acc.UpdateNickname)
	hallHandler := &hall.Handler{Pool: deps.Pool, Redis: deps.Redis}
	h.POST("/den/v1/hall/EnterHall", Authenticate(deps.Redis, "hall", "EnterHall"), hallHandler.EnterHall)
	roomStore := &room.Store{Pool: deps.Pool}
	rooms := &room.Handler{Store: roomStore, Publisher: deps.Publisher}
	h.POST("/den/v1/room/CreateRoom", Authenticate(deps.Redis, "room", "CreateRoom"), rooms.CreateRoom)
	h.POST("/den/v1/room/JoinRoom", Authenticate(deps.Redis, "room", "JoinRoom"), rooms.JoinRoom)
	h.POST("/den/v1/room/LeaveRoom", Authenticate(deps.Redis, "room", "LeaveRoom"), rooms.LeaveRoom)
	h.POST("/den/v1/room/Ready", Authenticate(deps.Redis, "room", "Ready"), rooms.Ready)
	h.POST("/den/v1/room/DescribeRoom", Authenticate(deps.Redis, "room", "DescribeRoom"), rooms.DescribeRoom)
	h.POST("/den/v1/room/StartGame", Authenticate(deps.Redis, "room", "StartGame"), rooms.StartGame)
	if deps.Hub != nil {
		h.GET("/den/v1/room/Subscribe", (&realtime.Handler{Hub: deps.Hub, Redis: deps.Redis, Pool: deps.Pool}).Subscribe)
	}
	diceHandler := &dice.Handler{
		Service:   &dice.Service{Pool: deps.Pool, Roller: roller},
		Publisher: deps.Publisher,
	}
	h.POST("/den/v1/dice/Roll", Authenticate(deps.Redis, "dice", "Roll"), diceHandler.Roll)
	h.POST("/den/v1/dice/Call", Authenticate(deps.Redis, "dice", "Call"), diceHandler.Call)
	h.POST("/den/v1/dice/Open", Authenticate(deps.Redis, "dice", "Open"), diceHandler.Open)
	h.POST("/den/v1/dice/DescribeRound", Authenticate(deps.Redis, "dice", "DescribeRound"), diceHandler.DescribeRound)
	matches := &match.Handler{Service: &match.Service{Pool: deps.Pool, Redis: deps.Redis, Rooms: roomStore}}
	h.POST("/den/v1/match/StartMatch", Authenticate(deps.Redis, "match", "StartMatch"), matches.StartMatch)
	h.POST("/den/v1/match/DescribeMatch", Authenticate(deps.Redis, "match", "DescribeMatch"), matches.DescribeMatch)
	h.POST("/den/v1/match/CancelMatch", Authenticate(deps.Redis, "match", "CancelMatch"), matches.CancelMatch)
}
