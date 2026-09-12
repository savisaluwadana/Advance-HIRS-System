package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

type invitationTokenRequest struct {
	Token string `json:"token"`
}

func (s *Server) listAccessInvitations(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	invitations, err := s.store.ListAccessInvitations(r.Context(), principal.OrganizationID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invitation_list_error", "could not load invitations")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": invitations, "total": len(invitations)})
}

func (s *Server) createAccessInvitation(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.CreateAccessInvitation
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.EmployeeID = strings.TrimSpace(input.EmployeeID)
	if input.Email == "" || !strings.Contains(input.Email, "@") {
		writeError(w, http.StatusBadRequest, "validation_error", "a valid email is required")
		return
	}
	if !workflowOneOf(input.Role, "admin", "hr", "manager", "employee") {
		writeError(w, http.StatusBadRequest, "validation_error", "role must be admin, hr, manager or employee")
		return
	}
	if principal.Role == "hr" && (input.Role == "admin" || input.Role == "hr") {
		writeError(w, http.StatusForbidden, "forbidden", "only administrators may grant admin or HR access")
		return
	}
	if (input.Role == "manager" || input.Role == "employee") && input.EmployeeID == "" {
		writeError(w, http.StatusBadRequest, "employee_link_required", "employee_id is required for manager and employee access")
		return
	}

	rawToken, tokenHash, err := newInvitationToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invitation_token_error", "could not generate invitation token")
		return
	}
	invitation, err := s.store.CreateAccessInvitation(
		r.Context(), principal.OrganizationID, principal.UserID, input.Email, input.Role,
		input.EmployeeID, tokenHash, time.Now().UTC().Add(48*time.Hour),
	)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusBadRequest, "employee_not_found", "employee does not exist in this organization")
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusBadRequest, "employee_email_mismatch", "invitation email must match the linked employee work email")
		default:
			writeError(w, http.StatusInternalServerError, "invitation_create_error", "could not create invitation")
		}
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "access.invited", "access_invitation", invitation.ID,
		workflowAuditMetadata(map[string]any{"email": invitation.Email, "role": invitation.Role, "employee_id": invitation.EmployeeID}))
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation": invitation,
		"token": rawToken,
		"notice": "This invitation token is shown only once. Share it through a trusted channel.",
	})
}

func (s *Server) revokeAccessInvitation(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	if err := s.store.RevokeAccessInvitation(r.Context(), principal.OrganizationID, r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "invitation_not_found", "pending invitation does not exist")
			return
		}
		writeError(w, http.StatusInternalServerError, "invitation_revoke_error", "could not revoke invitation")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "access.invitation_revoked", "access_invitation", r.PathValue("id"), `{}`)
	writeJSON(w, http.StatusOK, map[string]any{"revoked": true})
}

func (s *Server) inspectAccessInvitation(w http.ResponseWriter, r *http.Request) {
	var input invitationTokenRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(input.Token) == "" {
		writeError(w, http.StatusBadRequest, "token_required", "invitation token is required")
		return
	}
	preview, err := s.store.PreviewAccessInvitation(r.Context(), hashInvitationToken(input.Token))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "invitation_invalid", "invitation is invalid, expired, revoked or already used")
			return
		}
		writeError(w, http.StatusInternalServerError, "invitation_inspect_error", "could not inspect invitation")
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) acceptAccessInvitation(w http.ResponseWriter, r *http.Request) {
	var input model.AcceptInvitation
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Token = strings.TrimSpace(input.Token)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.Token == "" {
		writeError(w, http.StatusBadRequest, "token_required", "invitation token is required")
		return
	}
	identity, createdUser, err := s.store.AcceptAccessInvitation(r.Context(), hashInvitationToken(input.Token), input.DisplayName, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "invitation_invalid", "invitation is invalid, revoked or already used")
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "invitation_unavailable", "invitation expired or the account is disabled")
		default:
			// New-account password validation originates from auth.HashPassword.
			if strings.Contains(strings.ToLower(err.Error()), "password") {
				writeError(w, http.StatusBadRequest, "password_invalid", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "invitation_accept_error", "could not accept invitation")
		}
		return
	}

	token, expiresAt, err := s.auth.Issue(identity.UserID, identity.Email, identity.DisplayName, identity.OrganizationID, identity.Organization, identity.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", "could not create access token")
		return
	}
	s.store.RecordAudit(r.Context(), identity.OrganizationID, identity.UserID, "access.invitation_accepted", "organization_membership", identity.UserID,
		workflowAuditMetadata(map[string]any{"role": identity.Role, "new_user": createdUser}))
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type": "Bearer",
		"expires_at": expiresAt,
		"created_user": createdUser,
		"user": identity.Principal,
	})
}

func newInvitationToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(buffer)
	return raw, hashInvitationToken(raw), nil
}

func hashInvitationToken(raw string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(digest[:])
}
