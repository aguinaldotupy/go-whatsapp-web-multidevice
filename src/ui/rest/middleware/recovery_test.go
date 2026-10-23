package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow"
)

func TestRecovery_ValidationErrorReturns400(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery())

	app.Get("/panic-validation", func(c fiber.Ctx) error {
		panic(pkgError.ValidationError("invalid phone format"))
	})

	req := httptest.NewRequest("GET", "/panic-validation", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var res utils.ResponseData
	err = json.Unmarshal(bodyBytes, &res)
	require.NoError(t, err)
	assert.Equal(t, "VALIDATION_ERROR", res.Code)
	assert.Equal(t, "invalid phone format", res.Message)
}

func TestRecovery_NotFoundErrorReturns404(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery())

	app.Get("/panic-not-found", func(c fiber.Ctx) error {
		panic(pkgError.NotFoundError("message with ID 123 not found"))
	})

	req := httptest.NewRequest("GET", "/panic-not-found", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var res utils.ResponseData
	err = json.Unmarshal(bodyBytes, &res)
	require.NoError(t, err)
	assert.Equal(t, "NOT_FOUND", res.Code)
	assert.Equal(t, "message with ID 123 not found", res.Message)
}

func TestRecovery_UnexpectedPanicReturns500(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery())

	app.Get("/panic-crash", func(c fiber.Ctx) error {
		panic(errors.New("unexpected database nil pointer"))
	})

	req := httptest.NewRequest("GET", "/panic-crash", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var res utils.ResponseData
	err = json.Unmarshal(bodyBytes, &res)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", res.Code)
}

func TestRecovery_IQErrorRateOverlimitReturns429(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery())

	app.Get("/panic-rate-limit", func(c fiber.Ctx) error {
		panic(whatsmeow.ErrIQRateOverLimit)
	})

	req := httptest.NewRequest("GET", "/panic-rate-limit", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var res utils.ResponseData
	err = json.Unmarshal(bodyBytes, &res)
	require.NoError(t, err)
	assert.Equal(t, "RATE_OVERLIMIT", res.Code)
	assert.Contains(t, res.Message, "rate-overlimit")
}

func TestRecovery_IQErrorNotFoundReturns404(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery())

	app.Get("/panic-iq-not-found", func(c fiber.Ctx) error {
		panic(whatsmeow.ErrIQNotFound)
	})

	req := httptest.NewRequest("GET", "/panic-iq-not-found", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var res utils.ResponseData
	err = json.Unmarshal(bodyBytes, &res)
	require.NoError(t, err)
	assert.Equal(t, "ITEM_NOT_FOUND", res.Code)
}

func TestRecovery_DeadlineExceededReturns504(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery())

	app.Get("/panic-timeout", func(c fiber.Ctx) error {
		panic(context.DeadlineExceeded)
	})

	req := httptest.NewRequest("GET", "/panic-timeout", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 504, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var res utils.ResponseData
	err = json.Unmarshal(bodyBytes, &res)
	require.NoError(t, err)
	assert.Equal(t, "GATEWAY_TIMEOUT", res.Code)
}
