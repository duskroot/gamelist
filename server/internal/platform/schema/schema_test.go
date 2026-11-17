package schema_test

import (
	"context"
	"testing"

	"den/internal/platform/testbox"
)

func TestSchemaAcceptsUser(t *testing.T) {
	pool, rdb := testbox.Start(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO users(user_id, device_id, nickname) VALUES ('u1','dev-1','玩家0001')`); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, "den:token:tok", "u1", 0).Err(); err != nil {
		t.Fatal(err)
	}
	var nickname string
	if err := pool.QueryRow(ctx, `SELECT nickname FROM users WHERE device_id=$1`, "dev-1").Scan(&nickname); err != nil {
		t.Fatal(err)
	}
	if nickname != "玩家0001" {
		t.Fatalf("nickname %s", nickname)
	}
}
