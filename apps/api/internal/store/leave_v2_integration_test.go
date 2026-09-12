package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

func TestLeaveManagementV2Integration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	ctx := context.Background()
	dataStore, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()

	slug := fmt.Sprintf("leave-v2-%d", time.Now().UnixNano())
	orgID, err := dataStore.BootstrapV2(ctx, "Leave V2 Test", slug, "", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1::uuid`, orgID)

	if err := dataStore.EnsureDefaultLeavePolicies(ctx, orgID); err != nil {
		t.Fatal(err)
	}
	balances, err := dataStore.ListLeaveBalances(ctx, orgID, "emp_001", 2026)
	if err != nil {
		t.Fatal(err)
	}
	annual := findBalance(t, balances, "annual")
	if annual.Available != 20 {
		t.Fatalf("expected initial annual balance 20, got %.2f", annual.Available)
	}

	holiday, err := dataStore.CreateHoliday(ctx, orgID, model.CreateHoliday{
		Date: "2026-10-06", Name: "Company reset day", Location: "Colombo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if holiday.ID == "" {
		t.Fatal("expected holiday id")
	}

	start := mustDate(t, "2026-10-05")
	end := mustDate(t, "2026-10-07")
	request, err := dataStore.CreateLeaveRequestV2(ctx, orgID, "emp_001", "annual", start, end, "full", "integration test")
	if err != nil {
		t.Fatal(err)
	}
	if request.Days != 2 {
		t.Fatalf("expected 2 working days after holiday exclusion, got %.2f", request.Days)
	}

	_, err = dataStore.CreateLeaveRequestV2(ctx, orgID, "emp_001", "annual", mustDate(t, "2026-10-07"), mustDate(t, "2026-10-08"), "full", "overlap")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected overlap conflict, got %v", err)
	}

	approved, err := dataStore.DecideLeaveRequestV2(ctx, orgID, request.ID, "", "approved", "approved in integration test", "")
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "approved" {
		t.Fatalf("expected approved status, got %s", approved.Status)
	}
	balances, err = dataStore.ListLeaveBalances(ctx, orgID, "emp_001", 2026)
	if err != nil {
		t.Fatal(err)
	}
	annual = findBalance(t, balances, "annual")
	if annual.Available != 18 || annual.Used != 2 {
		t.Fatalf("expected annual balance 18 with 2 used, got available %.2f used %.2f", annual.Available, annual.Used)
	}

	cancelled, err := dataStore.CancelLeaveRequestV2(ctx, orgID, request.ID, "", "plans changed")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}
	balances, err = dataStore.ListLeaveBalances(ctx, orgID, "emp_001", 2026)
	if err != nil {
		t.Fatal(err)
	}
	annual = findBalance(t, balances, "annual")
	if annual.Available != 20 || annual.Used != 0 {
		t.Fatalf("expected cancellation to restore balance to 20, got available %.2f used %.2f", annual.Available, annual.Used)
	}

	half, err := dataStore.CreateLeaveRequestV2(ctx, orgID, "emp_001", "annual", mustDate(t, "2026-10-08"), mustDate(t, "2026-10-08"), "first_half", "appointment")
	if err != nil {
		t.Fatal(err)
	}
	if half.Days != 0.5 {
		t.Fatalf("expected half-day request to equal 0.5, got %.2f", half.Days)
	}

	large, err := dataStore.CreateLeaveRequestV2(ctx, orgID, "emp_002", "annual", mustDate(t, "2026-11-02"), mustDate(t, "2026-12-04"), "full", "too much leave")
	if err != nil {
		t.Fatal(err)
	}
	_, err = dataStore.DecideLeaveRequestV2(ctx, orgID, large.ID, "", "approved", "should fail", "")
	if !errors.Is(err, ErrInsufficientLeaveBalance) {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
}

func findBalance(t *testing.T, balances []model.LeaveBalance, leaveType string) model.LeaveBalance {
	t.Helper()
	for _, balance := range balances {
		if balance.LeaveType == leaveType {
			return balance
		}
	}
	t.Fatalf("missing %s balance", leaveType)
	return model.LeaveBalance{}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
