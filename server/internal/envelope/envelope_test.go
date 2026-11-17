package envelope

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

func TestOKEnvelope(t *testing.T) {
	h := server.New()
	h.POST("/den/v1/account/GuestLogin", func(_ context.Context, c *app.RequestContext) {
		OK(c, "account", "GuestLogin", map[string]string{"UserId": "u1"})
	})
	w := ut.PerformRequest(h.Engine, "POST", "/den/v1/account/GuestLogin", nil)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	meta := body["ResponseMetadata"].(map[string]any)
	if meta["Action"] != "GuestLogin" || meta["Version"] != "v1" || meta["Service"] != "account" {
		t.Fatalf("metadata %#v", meta)
	}
	if meta["RequestId"] == "" {
		t.Fatal("missing RequestId")
	}
	if _, ok := body["Error"]; ok {
		t.Fatal("success must not include Error")
	}
	if body["Result"].(map[string]any)["UserId"] != "u1" {
		t.Fatalf("result %#v", body["Result"])
	}
}

func TestFailOmitsResult(t *testing.T) {
	h := server.New()
	h.POST("/den/v1/room/CreateRoom", func(_ context.Context, c *app.RequestContext) {
		Fail(c, 200, "room", "CreateRoom", "100002", "Invalid Parameter", "MaxSeats must be between 2 and 6")
	})
	w := ut.PerformRequest(h.Engine, "POST", "/den/v1/room/CreateRoom", nil)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["Result"]; ok {
		t.Fatal("failure must not include Result")
	}
	errBody := body["Error"].(map[string]any)
	if errBody["Code"] != "100002" || errBody["Message"] != "Invalid Parameter" {
		t.Fatalf("error %#v", errBody)
	}
}

func TestPanicIsSystemError(t *testing.T) {
	h := server.New()
	h.Use(Recovery())
	h.GET("/panic", func(_ context.Context, c *app.RequestContext) {
		panic("boom")
	})
	w := ut.PerformRequest(h.Engine, "GET", "/panic", nil)
	if w.Code != 500 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	errBody := body["Error"].(map[string]any)
	if errBody["Code"] != "100001" || errBody["Message"] != "System Error" || errBody["Detail"] != nil && errBody["Detail"] != "" {
		t.Fatalf("error %#v", errBody)
	}
}
