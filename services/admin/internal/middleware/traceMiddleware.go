package middleware

import (
	"net/http"

	applog "fastgame/pkg/log"
)

func TraceMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return applog.HTTPMiddleware()
}
