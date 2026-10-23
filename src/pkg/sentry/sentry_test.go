package sentry

import (
	"errors"
	"testing"

	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow"
)

func TestInitSentryEmptyDSNDoesNothing(t *testing.T) {
	err := InitSentry("", "test", "v1.0.0", 1.0)
	assert.NoError(t, err)
}

func TestLogrusLevelMapping(t *testing.T) {
	assert.Equal(t, sentry.LevelFatal, logrusLevelToSentry(logrus.PanicLevel))
	assert.Equal(t, sentry.LevelFatal, logrusLevelToSentry(logrus.FatalLevel))
	assert.Equal(t, sentry.LevelError, logrusLevelToSentry(logrus.ErrorLevel))
	assert.Equal(t, sentry.LevelWarning, logrusLevelToSentry(logrus.WarnLevel))
	assert.Equal(t, sentry.LevelInfo, logrusLevelToSentry(logrus.InfoLevel))
	assert.Equal(t, sentry.LevelInfo, logrusLevelToSentry(logrus.DebugLevel))
}

func TestSentryLogrusHookLevels(t *testing.T) {
	hook := newSentryLogrusHook()
	require.NotNil(t, hook)
	assert.Contains(t, hook.Levels(), logrus.ErrorLevel)
	assert.Contains(t, hook.Levels(), logrus.FatalLevel)
	assert.Contains(t, hook.Levels(), logrus.PanicLevel)
	assert.NotContains(t, hook.Levels(), logrus.InfoLevel)
}

func TestBeforeSendFiltersValidationErrors(t *testing.T) {
	validationErr := pkgError.ValidationError("phone cannot be blank")
	genericErr := errors.New("unexpected database corruption")

	filter := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		if hint != nil && hint.OriginalException != nil {
			var genErr pkgError.GenericError
			if errors.As(hint.OriginalException, &genErr) {
				return nil
			}
		}
		return event
	}

	event := &sentry.Event{}
	// Validation error should be dropped (return nil)
	assert.Nil(t, filter(event, &sentry.EventHint{OriginalException: validationErr}))

	// Generic/system error should be preserved
	assert.NotNil(t, filter(event, &sentry.EventHint{OriginalException: genericErr}))
}

func TestBeforeSendFiltersIQErrors(t *testing.T) {
	rateLimitErr := whatsmeow.ErrIQRateOverLimit
	notFoundErr := whatsmeow.ErrIQNotFound
	serverErr := whatsmeow.ErrIQInternalServerError

	filter := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		if hint != nil && hint.OriginalException != nil {
			var genericErr pkgError.GenericError
			if errors.As(hint.OriginalException, &genericErr) && genericErr.StatusCode() < 500 {
				return nil
			}
			var iqErr *whatsmeow.IQError
			if errors.As(hint.OriginalException, &iqErr) && iqErr.Code < 500 {
				return nil
			}
		}
		return event
	}

	event := &sentry.Event{}
	// Rate limit (429) and Not Found (404) should be filtered out
	assert.Nil(t, filter(event, &sentry.EventHint{OriginalException: rateLimitErr}))
	assert.Nil(t, filter(event, &sentry.EventHint{OriginalException: notFoundErr}))

	// Upstream 500 internal server error should be kept
	assert.NotNil(t, filter(event, &sentry.EventHint{OriginalException: serverErr}))
}

func TestExtractException(t *testing.T) {
	validationErr := pkgError.ValidationError("limit: must be no greater than 100.")
	systemErr := errors.New("database failure")

	// Nil hint
	assert.Nil(t, extractException(nil))

	// OriginalException
	assert.Equal(t, validationErr, extractException(&sentry.EventHint{OriginalException: validationErr}))

	// RecoveredException (as error)
	assert.Equal(t, validationErr, extractException(&sentry.EventHint{RecoveredException: validationErr}))

	// RecoveredException (as non-error string)
	assert.Nil(t, extractException(&sentry.EventHint{RecoveredException: "some string"}))

	// Prefer OriginalException if both are set
	assert.Equal(t, systemErr, extractException(&sentry.EventHint{OriginalException: systemErr, RecoveredException: validationErr}))
}

func TestBeforeSendFiltersRecoveredValidationErrors(t *testing.T) {
	validationErr := pkgError.ValidationError("limit: must be no greater than 100.")
	systemErr := errors.New("database failure")

	filter := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		if ex := extractException(hint); ex != nil {
			var genericErr pkgError.GenericError
			if errors.As(ex, &genericErr) && genericErr.StatusCode() < 500 {
				return nil
			}
		}
		return event
	}

	event := &sentry.Event{}
	// Recovered validation error (e.g. from sentryfiber recovering a panic) should be dropped
	assert.Nil(t, filter(event, &sentry.EventHint{RecoveredException: validationErr}))

	// Recovered unexpected crash should be kept
	assert.NotNil(t, filter(event, &sentry.EventHint{RecoveredException: systemErr}))
}
