package middleware

import (
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
