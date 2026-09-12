package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/auth"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

type Server struct {
	store         *store.Store
	auth          *auth.Manager
	allowedOrigin string
}

type contextKey string

const principalKey contextKey = "principal"

type loginRequest struct {
	Email            string `json:"email"`
	Password         string `json:"password"`
	OrganizationSlug string `json:"organization_slug"`
}

type syncOperation struct {
	ID          string          `json:"id"`
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id"`
	Action      string          `json:"action"`
	Payload     json.RawMessage `json:"payload"`
	BaseVersion int64           `json:"base_version"`
	ChangedAt   time.Time       `json:"changed_at"`
}

type syncPushRequest struct {
	Operations []syncOperation `json:"operations"`
}

func NewHandler(store *store.Store, authManager *auth.Manager, allowedOrigin string) http.Handler {
	server := &Server{store: store, auth: authManager, allowedOrigin: allowedOrigin}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("POST /api/v1/auth/login", server.login)
	mux.HandleFunc("GET /api/v1/auth/me", server.authorize(server.me, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("GET /api/v1/dashboard", server.authorize(server.dashboard, "admin", "hr", "manager"))
	mux.HandleFunc("GET /api/v1/employees", server.authorize(server.listEmployees, "admin", "hr", "manager"))
	mux.HandleFunc("POST /api/v1/employees", server.authorize(server.createEmployee, "admin", "hr"))
	mux.HandleFunc("PATCH /api/v1/employees/{id}", server.authorize(server.patchEmployee, "admin", "hr"))
	mux.HandleFunc("DELETE /api/v1/employees/{id}", server.authorize(server.archiveEmployee, "admin", "hr"))
	mux.HandleFunc("GET /api/v1/leave/requests", server.authorize(server.leaveRequests, "admin", "hr", "manager"))
	mux.HandleFunc("POST /api/v1/sync/push", server.authorize(server.syncPush, "admin", "hr"))
	mux.HandleFunc("GET /api/v1/sync/pull", server.authorize(server.syncPull, "admin", "hr"))
	return server.withMiddleware(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	payload := map[string]any{
		"status":  "ok",
		"service": "advance-hris-api",
		"time":    time.Now().UTC(),
	}
	if err := s.store.Ready(r.Context()); err != nil {
		status = http.StatusServiceUnavailable
		payload["status"] = "degraded"
		payload["database"] = "unavailable"
	} else {
		payload["database"] = "ready"
	}
	writeJSON(w, status, payload)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	request.Email = strings.TrimSpace(strings.ToLower(request.Email))
	if request.Email == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "missing_credentials", "email and password are required")
		return
	}
	identity, err := s.store.LoginIdentity(r.Context(), request.Email, strings.TrimSpace(request.OrganizationSlug))
	if err != nil || !auth.CheckPassword(identity.PasswordHash, request.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		return
	}
	token, expiresAt, err := s.auth.Issue(identity.UserID, identity.Email, identity.DisplayName, identity.OrganizationID, identity.Organization, identity.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", "could not create access token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_at":   expiresAt,
		"user":         identity.Principal,
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": principal})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	metrics, err := s.store.Dashboard(r.Context(), principal.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "dashboard_error", "could not load dashboard")
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) listEmployees(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	employees, err := s.store.ListEmployees(r.Context(), principal.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "employee_list_error", "could not load employees")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": employees, "total": len(employees)})
}

func (s *Server) createEmployee(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.CreateEmployee
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(input.FirstName) == "" || strings.TrimSpace(input.LastName) == "" || strings.TrimSpace(input.WorkEmail) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "first_name, last_name and work_email are required")
		return
	}
	employee, err := s.store.CreateEmployee(r.Context(), principal.OrganizationID, input)
	if err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusConflict, "employee_conflict", "employee number or work email already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "employee_create_error", "could not create employee")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "employee.created", "employee", employee.ID, `{}`)
	writeJSON(w, http.StatusCreated, employee)
}

func (s *Server) patchEmployee(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var patch model.PatchEmployee
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	employee, err := s.store.PatchEmployee(r.Context(), principal.OrganizationID, r.PathValue("id"), patch)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "employee_not_found", "employee does not exist")
		case isConstraintError(err):
			writeError(w, http.StatusConflict, "employee_conflict", "employee number or work email already exists")
		default:
			writeError(w, http.StatusInternalServerError, "employee_update_error", "could not update employee")
		}
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "employee.updated", "employee", employee.ID, `{}`)
	writeJSON(w, http.StatusOK, employee)
}

func (s *Server) archiveEmployee(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	employee, err := s.store.ArchiveEmployee(r.Context(), principal.OrganizationID, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "employee_not_found", "employee does not exist")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "employee_archive_error", "could not archive employee")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "employee.archived", "employee", employee.ID, `{}`)
	writeJSON(w, http.StatusOK, employee)
}

func (s *Server) leaveRequests(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	requests, err := s.store.LeaveRequests(r.Context(), principal.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leave_list_error", "could not load leave requests")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": requests, "total": len(requests)})
}

func (s *Server) syncPush(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var request syncPushRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_sync_request", err.Error())
		return
	}
	accepted := make([]string, 0, len(request.Operations))
	conflicts := make([]model.SyncConflict, 0)
	for _, operation := range request.Operations {
		if operation.EntityType != "employee" || operation.Action != "upsert" || operation.EntityID == "" {
			continue
		}
		var incoming model.Employee
		if err := json.Unmarshal(operation.Payload, &incoming); err != nil {
			continue
		}
		if incoming.ID == "" {
			incoming.ID = operation.EntityID
		}
		updated, conflict, err := s.store.SyncUpsertEmployee(r.Context(), principal.OrganizationID, incoming, operation.BaseVersion)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "sync_error", "could not apply desktop changes")
			return
		}
		if conflict {
			conflicts = append(conflicts, model.SyncConflict{
				OperationID: operation.ID, EntityID: incoming.ID,
				ServerVersion: updated.ServerVersion, ServerRecord: updated,
			})
			continue
		}
		accepted = append(accepted, operation.ID)
		s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "employee.synced", "employee", incoming.ID, `{}`)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"accepted": accepted, "conflicts": conflicts, "server_time": time.Now().UTC(),
	})
}

func (s *Server) syncPull(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	employees, err := s.store.ListEmployees(r.Context(), principal.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sync_error", "could not load cloud snapshot")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"employees": employees,
		"cursor":    time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) authorize(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "bearer access token is required")
			return
		}
		raw := strings.TrimSpace(header[len("Bearer "):])
		claims, err := s.auth.Parse(raw)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "access token is invalid or expired")
			return
		}
		if _, ok := allowed[claims.Role]; !ok {
			writeError(w, http.StatusForbidden, "forbidden", "your role does not allow this action")
			return
		}
		principal := model.Principal{
			UserID: claims.UserID, Email: claims.Email, DisplayName: claims.DisplayName,
			OrganizationID: claims.OrganizationID, Organization: claims.Organization, Role: claims.Role,
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
	}
}

func principalFromContext(ctx context.Context) model.Principal {
	principal, _ := ctx.Value(principalKey).(model.Principal)
	return principal
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.allowedOrigin != "" && origin == s.allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("invalid JSON body: multiple values are not allowed")
	}
	return nil
}

func isConstraintError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23514" || pgErr.Code == "23503")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
