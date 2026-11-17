package envelope

import (
	"context"
	"log"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
)

type Metadata struct {
	RequestId string `json:"RequestId"`
	Action    string `json:"Action"`
	Version   string `json:"Version"`
	Service   string `json:"Service"`
}

type errorBody struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
	Detail  string `json:"Detail,omitempty"`
}

type success struct {
	ResponseMetadata Metadata `json:"ResponseMetadata"`
	Result           any      `json:"Result"`
}

type failure struct {
	ResponseMetadata Metadata  `json:"ResponseMetadata"`
	Error            errorBody `json:"Error"`
}

func meta(service, action string) Metadata {
	m := Metadata{RequestId: uuid.NewString(), Action: action, Version: "v1", Service: service}
	log.Printf("RequestId=%s Action=%s Version=%s Service=%s", m.RequestId, m.Action, m.Version, m.Service)
	return m
}

func OK(c *app.RequestContext, service, action string, result any) {
	c.JSON(consts.StatusOK, success{ResponseMetadata: meta(service, action), Result: result})
}

func Fail(c *app.RequestContext, status int, service, action, code, message, detail string) {
	c.JSON(status, failure{
		ResponseMetadata: meta(service, action),
		Error:            errorBody{Code: code, Message: message, Detail: detail},
	})
}

func Recovery() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic=%v", rec)
				Fail(c, consts.StatusInternalServerError, "den", "Unknown", "100001", "System Error", "")
			}
		}()
		c.Next(ctx)
	}
}
