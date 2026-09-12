package httpapi

import (
	"net/http"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/auth"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

// NewHandlerV4 preserves the secure access/onboarding surface from V3 and adds
// policy-backed leave accounting, holidays, balances and cancellation flows.
func NewHandlerV4(dataStore *store.Store, authManager *auth.Manager, allowedOrigin string) http.Handler {
	server := &Server{store: dataStore, auth: authManager, allowedOrigin: allowedOrigin}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("POST /api/v1/auth/login", server.login)
	mux.HandleFunc("GET /api/v1/auth/me", server.authorize(server.me, "admin", "hr", "manager", "employee"))

	mux.HandleFunc("POST /api/v1/access/invitations/inspect", server.inspectAccessInvitation)
	mux.HandleFunc("POST /api/v1/access/invitations/accept", server.acceptAccessInvitation)
	mux.HandleFunc("GET /api/v1/access/invitations", server.authorize(server.listAccessInvitations, "admin", "hr"))
	mux.HandleFunc("POST /api/v1/access/invitations", server.authorize(server.createAccessInvitation, "admin", "hr"))
	mux.HandleFunc("POST /api/v1/access/invitations/{id}/revoke", server.authorize(server.revokeAccessInvitation, "admin", "hr"))

	mux.HandleFunc("GET /api/v1/dashboard", server.authorize(server.dashboard, "admin", "hr", "manager"))
	mux.HandleFunc("GET /api/v1/employees", server.authorize(server.listEmployees, "admin", "hr", "manager"))
	mux.HandleFunc("POST /api/v1/employees", server.authorize(server.createEmployee, "admin", "hr"))
	mux.HandleFunc("PATCH /api/v1/employees/{id}", server.authorize(server.patchEmployee, "admin", "hr"))
	mux.HandleFunc("DELETE /api/v1/employees/{id}", server.authorize(server.archiveEmployee, "admin", "hr"))

	mux.HandleFunc("GET /api/v1/leave/policies", server.authorize(server.leavePoliciesV2, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("PUT /api/v1/leave/policies", server.authorize(server.upsertLeavePolicyV2, "admin", "hr"))
	mux.HandleFunc("POST /api/v1/leave/policies/{id}/assignments", server.authorize(server.assignLeavePolicyV2, "admin", "hr"))
	mux.HandleFunc("GET /api/v1/leave/balances", server.authorize(server.leaveBalancesV2, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("GET /api/v1/leave/balances/ledger", server.authorize(server.leaveBalanceLedgerV2, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/leave/balances/adjustments", server.authorize(server.adjustLeaveBalanceV2, "admin", "hr"))
	mux.HandleFunc("GET /api/v1/leave/holidays", server.authorize(server.holidaysV2, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/leave/holidays", server.authorize(server.createHolidayV2, "admin", "hr"))
	mux.HandleFunc("DELETE /api/v1/leave/holidays/{id}", server.authorize(server.deleteHolidayV2, "admin", "hr"))

	mux.HandleFunc("GET /api/v1/leave/requests", server.authorize(server.workflowLeaveRequests, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/leave/requests", server.authorize(server.createLeaveRequestV2, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/leave/requests/{id}/decision", server.authorize(server.decideLeaveRequestV2, "admin", "hr", "manager"))
	mux.HandleFunc("POST /api/v1/leave/requests/{id}/cancel", server.authorize(server.cancelLeaveRequestV2, "admin", "hr", "manager", "employee"))

	mux.HandleFunc("GET /api/v1/attendance", server.authorize(server.attendance, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/attendance/check-in", server.authorize(server.checkIn, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/attendance/check-out", server.authorize(server.checkOut, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("GET /api/v1/audit", server.authorize(server.auditEvents, "admin", "hr"))

	mux.HandleFunc("POST /api/v1/sync/push", server.authorize(server.syncPush, "admin", "hr"))
	mux.HandleFunc("GET /api/v1/sync/pull", server.authorize(server.syncPull, "admin", "hr"))
	return server.withMiddleware(mux)
}
