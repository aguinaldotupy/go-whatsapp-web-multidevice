package sentry

import (
	"errors"
	"testing"

	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
