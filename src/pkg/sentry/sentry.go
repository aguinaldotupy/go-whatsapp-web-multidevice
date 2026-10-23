package sentry

import (
	"errors"
	"time"

	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
)

// InitSentry initializes Sentry SDK and attaches a logrus hook if DSN is set.
func InitSentry(dsn, environment, release string, sampleRate float64) error {
	if dsn == "" {
		return nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          release,
		EnableTracing:    sampleRate > 0,
		TracesSampleRate: sampleRate,
		SendDefaultPII:   false,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if hint != nil && hint.OriginalException != nil {
				// Don't report client validation/rate-limit errors (4xx) to Sentry
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
		},
	})
	if err != nil {
		logrus.Warnf("Sentry initialization failed: %v", err)
		return err
	}

	logrus.AddHook(newSentryLogrusHook())
	logrus.Infof("Sentry initialized for environment: %s", environment)
	return nil
}

// Flush flushes any pending Sentry events within the timeout.
func Flush(timeout time.Duration) {
	sentry.Flush(timeout)
}

type sentryLogrusHook struct {
	levels []logrus.Level
}

func newSentryLogrusHook() *sentryLogrusHook {
	return &sentryLogrusHook{
		levels: []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel},
	}
}

func (h *sentryLogrusHook) Levels() []logrus.Level {
	return h.levels
}

func (h *sentryLogrusHook) Fire(entry *logrus.Entry) error {
	hub := sentry.CurrentHub()
	if hub == nil {
		return nil
	}

	hub.WithScope(func(scope *sentry.Scope) {
		if len(entry.Data) > 0 {
			scope.SetContext("logrus_fields", sentry.Context(entry.Data))
		}
		scope.SetLevel(logrusLevelToSentry(entry.Level))

		if errVal, ok := entry.Data[logrus.ErrorKey].(error); ok && errVal != nil {
			var genericErr pkgError.GenericError
			if errors.As(errVal, &genericErr) && genericErr.StatusCode() < 500 {
				return
			}
			var iqErr *whatsmeow.IQError
			if errors.As(errVal, &iqErr) && iqErr.Code < 500 {
				return
			}
			hub.CaptureException(errVal)
		} else {
			hub.CaptureMessage(entry.Message)
		}
	})
	return nil
}

func logrusLevelToSentry(level logrus.Level) sentry.Level {
	switch level {
	case logrus.PanicLevel, logrus.FatalLevel:
		return sentry.LevelFatal
	case logrus.ErrorLevel:
		return sentry.LevelError
	case logrus.WarnLevel:
		return sentry.LevelWarning
	default:
		return sentry.LevelInfo
	}
}
