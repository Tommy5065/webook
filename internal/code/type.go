package code

import "context"

type SDKService interface {
	Send(ctx context.Context, args []string, phoneNumber string) error
}

type TemplateParam struct {
	Code string `json:"code"`
	Min  string `json:"min"`
}
