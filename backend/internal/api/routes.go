package api

import (
	"errors"
	"net/http"
	apiai "ttgo/internal/api/ai"
	apianalytics "ttgo/internal/api/analytics"
	apiauth "ttgo/internal/api/auth"
	apibackups "ttgo/internal/api/backups"
	apicategories "ttgo/internal/api/categories"
	apicustomfields "ttgo/internal/api/customfields"
	apifolders "ttgo/internal/api/folders"
	apiconfluence "ttgo/internal/api/integrations/confluence"
	apijira "ttgo/internal/api/integrations/jira"
	apiqtest "ttgo/internal/api/integrations/qtest"
	apirequirements "ttgo/internal/api/requirements"
	apiruns "ttgo/internal/api/runs"
	apisearch "ttgo/internal/api/search"
	apitests "ttgo/internal/api/tests"
	apitokens "ttgo/internal/api/tokens"
	apiusers "ttgo/internal/api/users"
	apiwebhooks "ttgo/internal/api/webhooks"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"

	"github.com/go-pkgz/routegroup"
	"github.com/microcosm-cc/bluemonday"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func mountAPIRoutes(s *Server, api *routegroup.Bundle) {
	authHandler := apiauth.NewHandler(s.store, userFromContext)
	apiauth.Mount(api, authHandler, s.requireAuth)
	apifolders.Mount(api, apifolders.NewHandler(s.store, s.Hub), s.requireAuth)
	apicategories.Mount(api, apicategories.NewHandler(s.store), s.requireAuth)
	apitests.Mount(api, apitests.NewHandler(s.store, s.sanitizer), s.requireAuth)
	apirequirements.Mount(api, apirequirements.NewHandler(
		s.store,
		s.Hub,
		s.sanitizer,
		func(cfg *models.JiraConfig, sourceKey string, sanitizer *bluemonday.Policy) (title, description, sourceURL string, err error) {
			result := apijira.FetchTicket(cfg, sourceKey, sanitizer)
			if !result.Success {
				return "", "", "", errors.New(result.Error)
			}
			return result.Title, result.Description, result.URL, nil
		},
		apijira.FetchChildren,
		func(cfg *models.ConfluenceConfig, sourceKey string, sanitizer *bluemonday.Policy) (title, description, sourceURL string, err error) {
			page, err := apiconfluence.FetchConfluencePage(cfg, sourceKey, sanitizer)
			if err != nil {
				return "", "", "", err
			}
			return page.Title, page.BodyHTML, page.URL, nil
		},
		apijira.PostComment,
	), s.requireAuth)
	apijira.Mount(api, apijira.NewHandler(s.store, s.Hub, s.sanitizer), s.requireAuth, s.requireAdmin)
	apiconfluence.Mount(api, apiconfluence.NewHandler(s.store, s.Hub, s.sanitizer), s.requireAuth, s.requireAdmin)
	if s.aiHandler == nil {
		s.aiHandler = apiai.NewHandler(s.store, s.sanitizer)
	}
	apiai.Mount(api, s.aiHandler, s.requireAuth, s.requireAdmin)
	apiruns.Mount(api, apiruns.NewHandlerWithNotifier(s.store, s.Hub, s.notifyRunCompleted), s.requireAuth)
	apicustomfields.Mount(api, apicustomfields.NewHandler(s.store), s.requireAuth, s.requireAdmin)
	apisearch.Mount(api, apisearch.NewHandler(s.store), s.requireAuth)
	apitokens.Mount(api, apitokens.NewHandler(s.store), s.requireAdmin)
	apiwebhooks.Mount(api, apiwebhooks.NewHandler(s.store), s.requireAuth)
	apibackups.Mount(api, s.backups, s.requireAdmin)
	apiusers.Mount(api, apiusers.NewHandler(s.store, s.Hub, userFromContext, authHandler.LogAuthEvent), s.requireAdmin)
	apiqtest.Mount(api, apiqtest.NewHandler(s.store, s.Hub), s.requireAuth, s.requireAdmin)
	apianalytics.Mount(api, apianalytics.NewHandler(s.store), s.requireAuth)

	// Seed / demo data
	api.HandleFunc("GET /seed", s.requireAdmin(s.handleGetSeedStatus))
	api.HandleFunc("POST /seed", s.requireAdmin(s.handleCreateSeed))
	api.HandleFunc("DELETE /seed", s.requireAdmin(s.handleDeleteSeed))
	api.HandleFunc("DELETE /admin/reset", s.requireAdmin(s.handleResetAllData))

	apiws.Mount(api, s.Hub, s.store.ValidateSession, s.corsOrigin)
}

func mountSwaggerRoutes(mux *http.ServeMux) {
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
}
