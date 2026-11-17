package main

import (
	"context"
	"log"
	"os"

	"den/internal/httpapi"
	"den/internal/platform/schema"
	"den/internal/realtime"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	addr := env("DEN_HTTP_ADDR", ":8080")
	dsn := env("DEN_POSTGRES_DSN", "postgres://den:den@127.0.0.1:5432/den?sslmode=disable")
	redisAddr := env("DEN_REDIS_ADDR", "redis://127.0.0.1:6379/0")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := pool.Exec(ctx, schema.SQL); err != nil {
		log.Fatal(err)
	}
	opt, err := redis.ParseURL(redisAddr)
	if err != nil {
		log.Fatal(err)
	}
	h := httpapi.New(httpapi.Deps{
		Pool:  pool,
		Redis: redis.NewClient(opt),
		Hub:   realtime.NewHub(),
		Addr:  addr,
	})
	h.Spin()
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
