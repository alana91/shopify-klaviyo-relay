package api

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/alana91/shopify-klaviyo-relay/docs"
)

func HandleDocs() http.Handler {
	return httpSwagger.WrapHandler
}
