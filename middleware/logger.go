// middleware/logger.go
package middleware

import (
	"log"
	"net/http"
	"time"
)

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseWriterInterceptor) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Logger intercepte les requêtes pour afficher la méthode, le chemin, le statut et la durée
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		interceptor := &responseWriterInterceptor{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(interceptor, r)

		log.Printf(
			"[%s] %s | Statut: %d | Durée: %v",
			r.Method,
			r.URL.Path,
			interceptor.statusCode,
			time.Since(start),
		)
	})
}
