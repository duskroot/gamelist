package testbox

import (
	"context"
	"sync"
	"testing"
	"time"

	"den/internal/platform/schema"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

var (
	once   sync.Once
	pool   *pgxpool.Pool
	rdb    *redis.Client
	booted error
)

func Start(t *testing.T) (*pgxpool.Pool, *redis.Client) {
	t.Helper()
	once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("den"),
			tcpostgres.WithUsername("den"),
			tcpostgres.WithPassword("den"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			booted = err
			return
		}
		dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			booted = err
			return
		}
		pool, err = pgxpool.New(ctx, dsn)
		if err != nil {
			booted = err
			return
		}
		if _, err = pool.Exec(ctx, schema.SQL); err != nil {
			booted = err
			return
		}
		rc, err := tcredis.Run(ctx, "redis:7-alpine")
		if err != nil {
			booted = err
			return
		}
		url, err := rc.ConnectionString(ctx)
		if err != nil {
			booted = err
			return
		}
		opt, err := redis.ParseURL(url)
		if err != nil {
			booted = err
			return
		}
		rdb = redis.NewClient(opt)
	})
	if booted != nil {
		t.Fatal(booted)
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE dice_calls, dice_hands, dice_rounds, matches, room_seats, rooms, users`); err != nil {
		t.Fatal(err)
	}
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	return pool, rdb
}
