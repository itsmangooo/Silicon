package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/authorization"
)

func (a *API) communicationRoutes(mux *http.ServeMux) {
	const root = "/api/v1/organizations/{organizationID}"
	mux.Handle("GET "+root+"/messages", a.org(authorization.OrganizationRead, http.HandlerFunc(a.messages)))
	mux.Handle("POST "+root+"/messages", a.org(authorization.OrganizationRead, http.HandlerFunc(a.sendMessage)))
	mux.Handle("POST "+root+"/messages/{messageID}/read", a.org(authorization.OrganizationRead, http.HandlerFunc(a.readMessage)))
	mux.Handle("GET "+root+"/support", a.org(authorization.OrganizationRead, http.HandlerFunc(a.tickets)))
	mux.Handle("POST "+root+"/support", a.org(authorization.OrganizationRead, http.HandlerFunc(a.createTicket)))
	mux.Handle("GET "+root+"/support/{ticketID}", a.org(authorization.OrganizationRead, http.HandlerFunc(a.ticket)))
	mux.Handle("POST "+root+"/support/{ticketID}/events", a.org(authorization.OrganizationRead, http.HandlerFunc(a.updateTicket)))
	mux.Handle("GET /api/v1/account/notifications", a.auth(http.HandlerFunc(a.notifications)))
	mux.Handle("POST /api/v1/account/notifications/{notificationID}/read", a.auth(http.HandlerFunc(a.readNotification)))
}

func communicationOffset(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("offset")
	if raw == "" {
		return 0, true
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 || offset > 100000 {
		validation(w, "Invalid result offset.")
		return 0, false
	}
	return offset, true
}

func communicationPage[T any](rows []T) ([]T, bool) {
	if len(rows) > 50 {
		return rows[:50], true
	}
	return rows, false
}

func (a *API) messages(w http.ResponseWriter, r *http.Request) {
	offset, ok := communicationOffset(w, r)
	if !ok {
		return
	}
	folder, query := r.URL.Query().Get("folder"), strings.TrimSpace(r.URL.Query().Get("q"))
	if !oneOf(folder, "", "inbox", "sent") || len(query) > 160 {
		validation(w, "Choose inbox or sent and a search of up to 160 bytes.")
		return
	}
	items, err := a.repo.Messages(r.Context(), pathUUID(r, "organizationID"), currentUser(r.Context()).ID, folder == "sent", query, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	items, more := communicationPage(items)
	writeJSON(w, http.StatusOK, map[string]any{"messages": items, "hasMore": more})
}

func (a *API) sendMessage(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RecipientID uuid.UUID  `json:"recipientId"`
		ParentID    *uuid.UUID `json:"parentId"`
		Subject     string     `json:"subject"`
		Body        string     `json:"body"`
	}
	if !decode(w, r, &input) {
		return
	}
	user := currentUser(r.Context())
	input.Subject, input.Body = strings.TrimSpace(input.Subject), strings.TrimSpace(input.Body)
	if input.RecipientID == uuid.Nil || input.RecipientID == user.ID || !communicationText(input.Subject, 160) || !communicationText(input.Body, 8000) {
		validation(w, "Choose another organization member and enter a subject and message.")
		return
	}
	id, err := a.repo.SendMessage(r.Context(), pathUUID(r, "organizationID"), user.ID, input.RecipientID, input.ParentID, input.Subject, input.Body)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) readMessage(w http.ResponseWriter, r *http.Request) {
	item, err := a.repo.ReadMessage(r.Context(), pathUUID(r, "organizationID"), currentUser(r.Context()).ID, pathUUID(r, "messageID"))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) tickets(w http.ResponseWriter, r *http.Request) {
	offset, ok := communicationOffset(w, r)
	if !ok {
		return
	}
	status, query := r.URL.Query().Get("status"), strings.TrimSpace(r.URL.Query().Get("q"))
	if status != "" && !supportStatus(status) || len(query) > 160 {
		validation(w, "Choose a valid status and a search of up to 160 bytes.")
		return
	}
	user := currentUser(r.Context())
	items, err := a.repo.Tickets(r.Context(), pathUUID(r, "organizationID"), user.ID, user.IsSystemAdmin, query, status, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	items, more := communicationPage(items)
	writeJSON(w, http.StatusOK, map[string]any{"tickets": items, "hasMore": more})
}

func (a *API) createTicket(w http.ResponseWriter, r *http.Request) {
	var input struct{ Subject, Category, Body string }
	if !decode(w, r, &input) {
		return
	}
	input.Subject, input.Body = strings.TrimSpace(input.Subject), strings.TrimSpace(input.Body)
	if !oneOf(input.Category, "general", "account", "deployment", "server", "bug", "feature") || !communicationText(input.Subject, 160) || !communicationText(input.Body, 8000) {
		validation(w, "Enter a subject and description and choose a supported category.")
		return
	}
	id, err := a.repo.CreateTicket(r.Context(), pathUUID(r, "organizationID"), currentUser(r.Context()).ID, input.Subject, input.Category, input.Body)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) ticket(w http.ResponseWriter, r *http.Request) {
	offset, ok := communicationOffset(w, r)
	if !ok {
		return
	}
	user := currentUser(r.Context())
	item, events, err := a.repo.Ticket(r.Context(), pathUUID(r, "organizationID"), user.ID, pathUUID(r, "ticketID"), user.IsSystemAdmin, offset)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	events, more := communicationPage(events)
	writeJSON(w, http.StatusOK, map[string]any{"ticket": item, "events": events, "hasMore": more})
}

func (a *API) updateTicket(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Kind       string     `json:"kind"`
		Body       string     `json:"body"`
		AssigneeID *uuid.UUID `json:"assigneeId"`
	}
	if !decode(w, r, &input) {
		return
	}
	user := currentUser(r.Context())
	input.Body = strings.TrimSpace(input.Body)
	if !user.IsSystemAdmin && input.Kind != "reply" && (input.Kind != "status" || input.Body != "closed") {
		writeError(w, http.StatusForbidden, "support_staff_required", "Only installation support staff may make this change.")
		return
	}
	switch input.Kind {
	case "reply":
		if !communicationText(input.Body, 8000) {
			validation(w, "Enter a reply.")
			return
		}
	case "status":
		if !supportStatus(input.Body) {
			validation(w, "Choose a valid ticket status.")
			return
		}
	case "priority":
		if !oneOf(input.Body, "low", "normal", "high", "urgent") {
			validation(w, "Choose a valid priority.")
			return
		}
	case "assignment":
		input.Body = "unassigned"
		if input.AssigneeID != nil {
			input.Body = input.AssigneeID.String()
		}
	default:
		validation(w, "Choose a supported ticket update.")
		return
	}
	if input.Kind != "assignment" {
		input.AssigneeID = nil
	}
	err := a.repo.UpdateTicket(r.Context(), pathUUID(r, "organizationID"), user.ID, pathUUID(r, "ticketID"), user.IsSystemAdmin, input.Kind, input.Body, input.AssigneeID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) notifications(w http.ResponseWriter, r *http.Request) {
	items, unread, err := a.repo.Notifications(r.Context(), currentUser(r.Context()).ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": items, "unread": unread})
}

func (a *API) readNotification(w http.ResponseWriter, r *http.Request) {
	if err := a.repo.ReadNotification(r.Context(), currentUser(r.Context()).ID, pathUUID(r, "notificationID")); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func communicationText(value string, limit int) bool {
	return len(value) > 0 && len(value) <= limit && !strings.ContainsRune(value, 0)
}

func supportStatus(status string) bool {
	return oneOf(status, "open", "in_progress", "waiting_for_user", "resolved", "closed")
}
