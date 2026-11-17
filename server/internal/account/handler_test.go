package account_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"den/internal/httpapi"
	"den/internal/platform/testbox"
	"den/internal/realtime"

	"github.com/cloudwego/hertz/pkg/common/ut"
)

func TestGuestLoginSameDevice(t *testing.T) {
	pool, rdb := testbox.Start(t)
	h := httpapi.New(httpapi.Deps{Pool: pool, Redis: rdb, Publisher: realtime.Nop{}})
	body := []byte(`{"DeviceId":"11111111-1111-4111-8111-111111111111","Nickname":"阿甲"}`)
	w := ut.PerformRequest(h.Engine, "POST", "/den/v1/account/GuestLogin",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var first map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if _, ok := first["Error"]; ok {
		t.Fatalf("error %#v", first["Error"])
	}
	result := first["Result"].(map[string]any)
	userID := result["UserId"].(string)
	body = []byte(`{"DeviceId":"11111111-1111-4111-8111-111111111111"}`)
	w = ut.PerformRequest(h.Engine, "POST", "/den/v1/account/GuestLogin",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	var second map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second["Result"].(map[string]any)["UserId"] != userID {
		t.Fatalf("user changed %#v", second["Result"])
	}
	if second["Result"].(map[string]any)["Token"] == result["Token"] {
		t.Fatal("token must rotate")
	}
}
