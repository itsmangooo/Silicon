package httpapi

import (
	"net/http"
	"strings"

	"github.com/itsmangooo/Silicon/backend/internal/authorization"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) < 2 || len(query) > 100 {
		validation(w, "Search queries must contain between 2 and 100 characters.")
		return
	}
	role, _ := r.Context().Value(roleKey).(string)
	results, err := a.repo.Search(r.Context(), pathUUID(r, "organizationID"), query, store.SearchOptions{
		IncludeMembers: authorization.Allowed(role, authorization.MemberRead),
		IncludeBudgets: authorization.Allowed(role, authorization.BudgetRead),
	})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": query, "results": results})
}
