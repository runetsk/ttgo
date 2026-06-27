package requirements

import (
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
)

type JiraFetcher func(cfg *models.JiraConfig, sourceKey string, sanitizer *bluemonday.Policy) (title, description, sourceURL string, err error)
type JiraChildrenFetcher func(cfg *models.JiraConfig, parentKey string, sanitizer *bluemonday.Policy) []models.JiraTicketChild
type ConfluenceFetcher func(cfg *models.ConfluenceConfig, sourceKey string, sanitizer *bluemonday.Policy) (title, description, sourceURL string, err error)
type JiraCommentPoster func(cfg *models.JiraConfig, issueKey string, adfBody interface{}) error

type Handler struct {
	store             *store.Store
	hub               *apiws.Hub
	sanitizer         *bluemonday.Policy
	fetchJiraTicket   JiraFetcher
	fetchJiraChildren JiraChildrenFetcher
	fetchConfluence   ConfluenceFetcher
	postJiraComment   JiraCommentPoster
}

func NewHandler(s *store.Store, hub *apiws.Hub, sanitizer *bluemonday.Policy, fetchJiraTicket JiraFetcher, fetchJiraChildren JiraChildrenFetcher, fetchConfluence ConfluenceFetcher, postJiraComment JiraCommentPoster) *Handler {
	return &Handler{
		store:             s,
		hub:               hub,
		sanitizer:         sanitizer,
		fetchJiraTicket:   fetchJiraTicket,
		fetchJiraChildren: fetchJiraChildren,
		fetchConfluence:   fetchConfluence,
		postJiraComment:   postJiraComment,
	}
}
