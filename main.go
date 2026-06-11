package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprintln(w, "shopify-klaviyo-relay"); err != nil {
			slog.Error("write response", "error", err)
		}
	})

	slog.Info("listening", "addr", ":8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
