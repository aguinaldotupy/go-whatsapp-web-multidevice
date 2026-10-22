package cmd

import (
	"net/http/httptest"
	"testing"
	"time"

	pkgSentry "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/sentry"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	sentryfiber "github.com/getsentry/sentry-go/fiberv3"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentryMiddlewareCatchesPanicAndRecoveryHandlesResponse(t *testing.T) {
	// Initialize Sentry with a mock DSN
	err := pkgSentry.InitSentry("https://key@o123456.ingest.sentry.io/123456", "test", "v1.0.0", 1.0)
	require.NoError(t, err)
	defer pkgSentry.Flush(100 * time.Millisecond)

	app := fiber.New()
	app.Use(middleware.Recovery())
	app.Use(sentryfiber.New(sentryfiber.Options{
		Repanic: true,
	}))

	app.Get("/panic-test", func(c fiber.Ctx) error {
		panic("test panic for sentry")
	})

	req := httptest.NewRequest("GET", "/panic-test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}
