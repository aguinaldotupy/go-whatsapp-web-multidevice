package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
)

func Recovery() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		defer func() {
			err := recover()
			if err != nil {
				var res utils.ResponseData
				res.Status = 500
				res.Code = "INTERNAL_SERVER_ERROR"
				res.Message = fmt.Sprintf("%v", err)

				var errErr error
				if e, ok := err.(error); ok {
					errErr = e
				} else {
					errErr = fmt.Errorf("%v", err)
				}

				var genericErr pkgError.GenericError
				var iqErr *whatsmeow.IQError

				switch {
				case errors.Is(errErr, context.DeadlineExceeded):
					res.Status = 504
					res.Code = "GATEWAY_TIMEOUT"
					res.Message = "Request timed out waiting for WhatsApp server response"
					logrus.Warnf("Gateway timeout recovered in middleware: %v", err)

				case errors.As(errErr, &genericErr):
					res.Status = genericErr.StatusCode()
					res.Code = genericErr.ErrCode()
					res.Message = genericErr.Error()
					if res.Status >= 500 {
						logrus.Errorf("Server error recovered in middleware: %v", err)
					} else {
						logrus.Warnf("Client error recovered in middleware: %v", err)
					}

				case errors.As(errErr, &iqErr):
					res.Status = iqErr.Code
					switch iqErr.Code {
					case 429:
						res.Code = "RATE_OVERLIMIT"
						res.Message = "WhatsApp rate limit exceeded (rate-overlimit). Please reduce request rate and try again."
					case 404:
						res.Code = "ITEM_NOT_FOUND"
						res.Message = "WhatsApp item not found: " + iqErr.Text
					case 401, 403:
						res.Code = "NOT_AUTHORIZED"
						res.Message = "WhatsApp query not authorized: " + iqErr.Text
					default:
						if iqErr.Code >= 400 && iqErr.Code < 500 {
							code := strings.ToUpper(strings.ReplaceAll(iqErr.Text, "-", "_"))
							if code == "" {
								code = "CLIENT_ERROR"
							}
							res.Code = code
							res.Message = fmt.Sprintf("WhatsApp client error %d: %s", iqErr.Code, iqErr.Text)
						} else {
							res.Status = 502
							res.Code = "BAD_GATEWAY"
							res.Message = fmt.Sprintf("WhatsApp upstream error %d: %s", iqErr.Code, iqErr.Text)
						}
					}
					if res.Status >= 500 {
						logrus.Errorf("WhatsApp upstream error recovered in middleware: %v", err)
					} else {
						logrus.Warnf("WhatsApp client error recovered in middleware: %v", err)
					}

				default:
					logrus.Errorf("Panic recovered in middleware: %v", err)
				}

				_ = ctx.Status(res.Status).JSON(res)
			}
		}()

		return ctx.Next()
	}
}
