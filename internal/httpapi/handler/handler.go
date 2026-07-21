package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service"
)

type Handler struct {
	mux     *http.ServeMux
	authSvc *service.AuthService
}

func NewHandler(repo repository.Repository) *Handler {
	h := &Handler{mux: http.NewServeMux(), authSvc: service.NewAuthService(repo)}

	eventService := service.NewEventService(repo)
	timetableService := service.NewTimetableService(repo)
	colonyService := service.NewColonyService(repo)
	analysisJobService := service.NewAnalysisJobService()

	authWrap := h.withAuth
	noAuth := func(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			next(w, r)
		}
	}

	h.mux.HandleFunc("/v1/events", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.listEvents(w, r, eventService)
		case http.MethodPost:
			h.createEvent(w, r, eventService)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
	}))

	h.mux.HandleFunc("/v1/events/", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.getEvent(w, r, eventService)
		case http.MethodPatch:
			h.updateEvent(w, r, eventService)
		case http.MethodDelete:
			h.deleteEvent(w, r, eventService)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
	}))

	h.mux.HandleFunc("/v1/timetable-entries", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.listTimetableEntries(w, r, timetableService)
		case http.MethodPost:
			h.createTimetableEntry(w, r, timetableService)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
	}))

	h.mux.HandleFunc("/v1/timetable-entries/", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.getTimetableEntry(w, r, timetableService)
		case http.MethodPatch:
			h.updateTimetableEntry(w, r, timetableService)
		case http.MethodDelete:
			h.deleteTimetableEntry(w, r, timetableService)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
	}))

	h.mux.HandleFunc("/v1/colonies", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.listColonies(w, r, colonyService)
		case http.MethodPost:
			h.createColony(w, r, colonyService)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
	}))

	h.mux.HandleFunc("/v1/colonies/", authWrap(func(w http.ResponseWriter, r *http.Request) {
		h.handleColonySubroutes(w, r, colonyService)
	}))

	h.mux.HandleFunc("/v1/auth/sessions", noAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.createSession(w, r, h.authSvc)
		case http.MethodDelete:
			h.deleteSession(w, r, h.authSvc)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
	}))

	h.mux.HandleFunc("/v1/auth/sessions/", noAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
			return
		}
		h.deleteSession(w, r, h.authSvc)
	}))

	h.mux.HandleFunc("/v1/uploads/jobs", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.createAnalysisJob(w, r, analysisJobService)
		case http.MethodGet:
			h.listAnalysisJobs(w, r, analysisJobService)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
	}))

	h.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	h.mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) resolveUserID(r *http.Request) string {
	userID := h.authSvc.ResolveUserID(r.Header.Get("X-User-ID"), r.Header.Get("Authorization"))
	if userID == "" {
		if cookie, err := r.Cookie("session"); err == nil {
			if session, err := h.authSvc.Repo().GetSessionByToken(cookie.Value); err == nil {
				userID = session.UserID
			}
		}
	}
	return userID
}

func (h *Handler) withAuth(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := h.resolveUserID(r)
		if userID == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "UNAUTHORIZED"}})
			return
		}
		r.Header.Set("X-User-ID", userID)
		next(w, r)
	}
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request, svc *service.EventService) {
	userID := h.resolveUserID(r)
	items, err := svc.List(userID, "", 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeEvents(items)})
}

func (h *Handler) createEvent(w http.ResponseWriter, r *http.Request, svc *service.EventService) {
	userID := h.resolveUserID(r)
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		StartAt     string `json:"startAt"`
		EndAt       string `json:"endAt"`
		AllDay      bool   `json:"allDay"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		if err.Error() == "EOF" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	startAt, err := parseTime(input.StartAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	endAt, err := parseTime(input.EndAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	item, err := svc.Create(userID, service.CreateEventInput{Title: input.Title, Description: input.Description, StartAt: startAt, EndAt: endAt, AllDay: input.AllDay})
	if err != nil {
		if repository.IsValidationError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "CONFLICT"}})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": serializeEvent(item)})
}

func (h *Handler) getEvent(w http.ResponseWriter, r *http.Request, svc *service.EventService) {
	userID := h.resolveUserID(r)
	eventID := strings.TrimPrefix(r.URL.Path, "/v1/events/")
	item, err := svc.Get(userID, eventID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeEvent(item)})
}

func (h *Handler) updateEvent(w http.ResponseWriter, r *http.Request, svc *service.EventService) {
	userID := h.resolveUserID(r)
	eventID := strings.TrimPrefix(r.URL.Path, "/v1/events/")
	var input struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		StartAt     *string `json:"startAt"`
		EndAt       *string `json:"endAt"`
		AllDay      *bool   `json:"allDay"`
		Version     int     `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		if err.Error() == "EOF" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	var startAt *time.Time
	var endAt *time.Time
	if input.StartAt != nil {
		parsed, err := parseTime(*input.StartAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		startAt = &parsed
	}
	if input.EndAt != nil {
		parsed, err := parseTime(*input.EndAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		endAt = &parsed
	}
	item, err := svc.Update(userID, eventID, service.UpdateEventInput{Title: input.Title, Description: input.Description, StartAt: startAt, EndAt: endAt, AllDay: input.AllDay, Version: input.Version})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "CONFLICT"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeEvent(item)})
}

func (h *Handler) deleteEvent(w http.ResponseWriter, r *http.Request, svc *service.EventService) {
	userID := h.resolveUserID(r)
	eventID := strings.TrimPrefix(r.URL.Path, "/v1/events/")
	if err := svc.Delete(userID, eventID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) listTimetableEntries(w http.ResponseWriter, r *http.Request, svc *service.TimetableService) {
	userID := h.resolveUserID(r)
	items, err := svc.List(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeTimetableEntries(items)})
}

func (h *Handler) createTimetableEntry(w http.ResponseWriter, r *http.Request, svc *service.TimetableService) {
	userID := h.resolveUserID(r)
	var input struct {
		DayOfWeek int    `json:"dayOfWeek"`
		Period    int    `json:"period"`
		Subject   string `json:"subject"`
		Room      string `json:"room"`
		Teacher   string `json:"teacher"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		if err.Error() == "EOF" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	item, err := svc.Create(userID, service.CreateTimetableEntryInput{DayOfWeek: input.DayOfWeek, Period: input.Period, Subject: input.Subject, Room: input.Room, Teacher: input.Teacher})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "CONFLICT"}})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": serializeTimetableEntry(item)})
}

func (h *Handler) getTimetableEntry(w http.ResponseWriter, r *http.Request, svc *service.TimetableService) {
	userID := h.resolveUserID(r)
	entryID := strings.TrimPrefix(r.URL.Path, "/v1/timetable-entries/")
	item, err := svc.Get(userID, entryID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeTimetableEntry(item)})
}

func (h *Handler) updateTimetableEntry(w http.ResponseWriter, r *http.Request, svc *service.TimetableService) {
	userID := h.resolveUserID(r)
	entryID := strings.TrimPrefix(r.URL.Path, "/v1/timetable-entries/")
	var input struct {
		DayOfWeek *int    `json:"dayOfWeek"`
		Period    *int    `json:"period"`
		Subject   *string `json:"subject"`
		Room      *string `json:"room"`
		Teacher   *string `json:"teacher"`
		Version   int     `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		if err.Error() == "EOF" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	item, err := svc.Update(userID, entryID, service.UpdateTimetableEntryInput{DayOfWeek: input.DayOfWeek, Period: input.Period, Subject: input.Subject, Room: input.Room, Teacher: input.Teacher, Version: input.Version})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "CONFLICT"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeTimetableEntry(item)})
}

func (h *Handler) deleteTimetableEntry(w http.ResponseWriter, r *http.Request, svc *service.TimetableService) {
	userID := h.resolveUserID(r)
	entryID := strings.TrimPrefix(r.URL.Path, "/v1/timetable-entries/")
	if err := svc.Delete(userID, entryID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) listColonies(w http.ResponseWriter, r *http.Request, svc *service.ColonyService) {
	userID := h.resolveUserID(r)
	items, err := svc.List(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeColonies(items)})
}

func (h *Handler) createColony(w http.ResponseWriter, r *http.Request, svc *service.ColonyService) {
	userID := h.resolveUserID(r)
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	item, err := svc.Create(userID, service.CreateColonyInput{Name: input.Name, Description: input.Description})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "CONFLICT"}})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": serializeColony(item)})
}

func (h *Handler) handleColonySubroutes(w http.ResponseWriter, r *http.Request, svc *service.ColonyService) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/colonies/"), "/")
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}

	parts := strings.Split(path, "/")

	if len(parts) == 1 {
		id := parts[0]
		if id == "members" || id == "feed" || id == "join" || id == "leave" {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
			return
		}
		switch r.Method {
		case http.MethodGet:
			userID := h.resolveUserID(r)
			item, err := svc.Get(userID, id)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": serializeColony(item)})
		case http.MethodPatch:
			userID := h.resolveUserID(r)
			var input struct {
				Name        *string `json:"name"`
				Description *string `json:"description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
				return
			}
			item, err := svc.Update(userID, id, service.UpdateColonyInput{Name: input.Name, Description: input.Description})
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": serializeColony(item)})
		case http.MethodDelete:
			userID := h.resolveUserID(r)
			if err := svc.Delete(userID, id); err != nil {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "FORBIDDEN"}})
				return
			}
			writeJSON(w, http.StatusNoContent, nil)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		}
		return
	}

	if len(parts) == 2 {
		colonyID := parts[0]
		switch parts[1] {
		case "members":
			userID := h.resolveUserID(r)
			h.listColonyMembers(w, r, svc, userID, colonyID)
			return
		case "feed":
			userID := h.resolveUserID(r)
			h.listColonyFeed(w, r, svc, userID, colonyID)
			return
		case "join":
			h.joinColony(w, r, svc, colonyID)
			return
		case "leave":
			h.leaveColony(w, r, svc, colonyID)
			return
		case "shared-items":
			h.handleSharedItems(w, r, svc, colonyID)
			return
		}
	}

	if len(parts) == 3 && parts[1] == "shared-items" {
		h.handleSharedItemDelete(w, r, svc, parts[0], parts[2])
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
}

func (h *Handler) listColonyMembers(w http.ResponseWriter, r *http.Request, svc *service.ColonyService, userID, colonyID string) {
	_, err := svc.Get(userID, colonyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	members, err := svc.ListMembers(colonyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR"}})
		return
	}
	out := make([]map[string]any, 0, len(members))
	for _, m := range members {
		out = append(out, map[string]any{"colonyId": m.ColonyID, "userId": m.UserID, "role": m.Role, "joinedAt": m.JoinedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) listColonyFeed(w http.ResponseWriter, r *http.Request, svc *service.ColonyService, userID, colonyID string) {
	_, err := svc.Get(userID, colonyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	items, err := svc.Feed(colonyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeSharedItems(items)})
}

func (h *Handler) joinColony(w http.ResponseWriter, r *http.Request, svc *service.ColonyService, colonyID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		return
	}
	userID := h.resolveUserID(r)
	var input struct {
		InviteCode string `json:"inviteCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input.InviteCode = ""
	}
	_, err := svc.Join(userID, colonyID, input.InviteCode)
	if err != nil {
		if err == repository.ErrForbidden {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "FORBIDDEN"}})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"ok": true}})
}

func (h *Handler) leaveColony(w http.ResponseWriter, r *http.Request, svc *service.ColonyService, colonyID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		return
	}
	userID := h.resolveUserID(r)
	if err := svc.Leave(userID, colonyID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"ok": true}})
}

func (h *Handler) handleSharedItems(w http.ResponseWriter, r *http.Request, svc *service.ColonyService, colonyID string) {
	userID := h.resolveUserID(r)
	if colonyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	switch r.Method {
	case http.MethodPost:
		var input struct {
			SourceType string `json:"sourceType"`
			SourceID   string `json:"sourceId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
			return
		}
		item, err := svc.CreateSharedItem(userID, colonyID, service.CreateSharedItemInput{SourceType: input.SourceType, SourceID: input.SourceID})
		if err != nil {
			if err == repository.ErrDuplicate {
				writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "CONFLICT"}})
				return
			}
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": serializeSharedItem(item)})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
	}
}

func (h *Handler) handleSharedItemDelete(w http.ResponseWriter, r *http.Request, svc *service.ColonyService, colonyID, sharedItemID string) {
	userID := h.resolveUserID(r)
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "METHOD_NOT_ALLOWED"}})
		return
	}
	if err := svc.DeleteSharedItem(userID, colonyID, sharedItemID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request, svc *service.AuthService) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	var payload map[string]json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			payload = map[string]json.RawMessage{}
		}
	}
	var userID, name string
	for _, key := range []string{"userId", "userID", "user_id"} {
		if raw, ok := payload[key]; ok {
			var value string
			if err := json.Unmarshal(raw, &value); err == nil && value != "" {
				userID = value
				break
			}
		}
	}
	if raw, ok := payload["name"]; ok {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			name = value
		}
	}
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	session, err := svc.CreateSession(userID, name)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "FORBIDDEN"}})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"id": session.ID, "userId": session.UserID, "token": session.Token, "name": session.Name}})
}

func (h *Handler) deleteSession(w http.ResponseWriter, r *http.Request, svc *service.AuthService) {
	token := strings.TrimPrefix(r.URL.Path, "/v1/auth/sessions")
	if token == "" || token == "/" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "VALIDATION_ERROR"}})
		return
	}
	if err := svc.Logout(strings.TrimPrefix(token, "/")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND"}})
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) createAnalysisJob(w http.ResponseWriter, r *http.Request, svc *service.AnalysisJobService) {
	userID := h.resolveUserID(r)
	var input struct {
		ContentType string `json:"contentType"`
		Filename    string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input = struct {
			ContentType string `json:"contentType"`
			Filename    string `json:"filename"`
		}{}
	}
	job, err := svc.CreateJob(userID, input.ContentType, input.Filename)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR"}})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"id": job.ID, "status": job.Status, "filename": job.Filename, "contentType": job.ContentType, "userId": job.UserID}})
}

func (h *Handler) listAnalysisJobs(w http.ResponseWriter, r *http.Request, svc *service.AnalysisJobService) {
	userID := h.resolveUserID(r)
	jobs, err := svc.ListJobs(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "INTERNAL_ERROR"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": jobs})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func serializeEvent(item repository.Event) map[string]any {
	return map[string]any{"id": item.ID, "title": item.Title, "description": item.Description, "startAt": item.StartAt.Format(time.RFC3339), "endAt": item.EndAt.Format(time.RFC3339), "allDay": item.AllDay, "version": item.Version}
}

func serializeEvents(items []repository.Event) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeEvent(item))
	}
	return out
}

func serializeTimetableEntry(item repository.TimetableEntry) map[string]any {
	return map[string]any{"id": item.ID, "dayOfWeek": item.DayOfWeek, "period": item.Period, "subject": item.Subject, "room": item.Room, "teacher": item.Teacher, "version": item.Version}
}

func serializeTimetableEntries(items []repository.TimetableEntry) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeTimetableEntry(item))
	}
	return out
}

func serializeColony(item repository.Colony) map[string]any {
	return map[string]any{"id": item.ID, "name": item.Name, "description": item.Description, "ownerUserId": item.OwnerUserID, "inviteCode": item.InviteCode}
}

func serializeColonies(items []repository.Colony) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeColony(item))
	}
	return out
}

func serializeSharedItem(item repository.SharedItem) map[string]any {
	return map[string]any{"id": item.ID, "colonyId": item.ColonyID, "sourceType": item.SourceType, "sourceId": item.SourceID, "createdBy": item.CreatedBy, "titleSnapshot": item.TitleSnapshot}
}

func serializeSharedItems(items []repository.SharedItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeSharedItem(item))
	}
	return out
}
