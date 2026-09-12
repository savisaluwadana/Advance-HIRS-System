package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

func TestAttendanceV2Integration(t *testing.T) {
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

	orgID, err := dataStore.BootstrapV2(ctx, "Attendance V2 Test", "attendance-v2-test", "", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1::uuid`, orgID)

	if err := dataStore.EnsureDefaultWorkSchedule(ctx, orgID); err != nil {
		t.Fatal(err)
	}
	schedule, err := dataStore.UpsertWorkSchedule(ctx, orgID, model.UpsertWorkSchedule{
		Code: "colombo-core", Name: "Colombo core", Timezone: "Asia/Colombo",
		StartTime: "09:00", EndTime: "17:00", BreakMinutes: 60,
		GraceMinutes: 10, OvertimeThresholdMinutes: 15,
		WorkDays: []int{1, 2, 3, 4, 5}, IsDefault: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if schedule.Timezone != "Asia/Colombo" || schedule.StartTime != "09:00" {
		t.Fatalf("unexpected schedule: %#v", schedule)
	}
	if err := dataStore.AssignWorkScheduleSafe(ctx, orgID, schedule.ID, model.WorkScheduleAssignment{
		EmployeeID: "emp_001", ScheduleID: schedule.ID, EffectiveFrom: "2026-09-01",
	}); err != nil {
		t.Fatal(err)
	}

	alternate, err := dataStore.UpsertWorkSchedule(ctx, orgID, model.UpsertWorkSchedule{
		Code: "late-shift", Name: "Late shift", Timezone: "Asia/Colombo",
		StartTime: "11:00", EndTime: "19:00", BreakMinutes: 60,
		GraceMinutes: 5, OvertimeThresholdMinutes: 15,
		WorkDays: []int{1, 2, 3, 4, 5}, IsDefault: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = dataStore.AssignWorkScheduleSafe(ctx, orgID, alternate.ID, model.WorkScheduleAssignment{
		EmployeeID: "emp_001", ScheduleID: alternate.ID, EffectiveFrom: "2026-09-15",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected overlapping schedule conflict, got %v", err)
	}

	checkIn := mustRFC3339Attendance(t, "2026-09-14T03:50:00Z") // 09:20 Asia/Colombo
	checkOut := mustRFC3339Attendance(t, "2026-09-14T13:00:00Z") // 18:30 Asia/Colombo
	correction, err := dataStore.CreateAttendanceCorrection(ctx, orgID, "emp_001", "", model.CreateAttendanceCorrection{
		WorkDate: "2026-09-14", WorkMode: "office", Reason: "Forgot to clock in",
	}, &checkIn, &checkOut)
	if err != nil {
		t.Fatal(err)
	}
	if correction.Status != "pending" {
		t.Fatalf("expected pending correction, got %s", correction.Status)
	}

	approved, err := dataStore.DecideAttendanceCorrection(ctx, orgID, correction.ID, "", "approved", "verified")
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "approved" || approved.AttendanceEntryID == "" {
		t.Fatalf("expected approved correction linked to attendance, got %#v", approved)
	}

	entries, err := dataStore.ListAttendanceV2(ctx, orgID, mustDateAttendance(t, "2026-09-14"), "emp_001", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one attendance entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.ScheduleID != schedule.ID || entry.ScheduleName != "Colombo core" {
		t.Fatalf("expected assigned schedule snapshot, got %#v", entry)
	}
	if entry.LateMinutes != 10 {
		t.Fatalf("expected 10 late minutes, got %d", entry.LateMinutes)
	}
	if entry.WorkedMinutes != 490 {
		t.Fatalf("expected 490 worked minutes, got %d", entry.WorkedMinutes)
	}
	if entry.OvertimeMinutes != 55 {
		t.Fatalf("expected 55 overtime minutes, got %d", entry.OvertimeMinutes)
	}
	if entry.EarlyLeaveMinutes != 0 || entry.Source != "correction" {
		t.Fatalf("unexpected computed attendance fields: %#v", entry)
	}

	summary, err := dataStore.AttendanceTimesheet(ctx, orgID, "emp_001", mustDateAttendance(t, "2026-09-14"), mustDateAttendance(t, "2026-09-18"))
	if err != nil {
		t.Fatal(err)
	}
	if summary.ScheduledDays != 5 || summary.RecordedDays != 1 {
		t.Fatalf("expected 5 scheduled / 1 recorded day, got %d / %d", summary.ScheduledDays, summary.RecordedDays)
	}
	if summary.WorkedMinutes != 490 || summary.OvertimeMinutes != 55 || summary.LateMinutes != 10 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.WorkedHours != 8.17 || summary.OvertimeHours != 0.92 {
		t.Fatalf("unexpected rounded hours: worked %.2f overtime %.2f", summary.WorkedHours, summary.OvertimeHours)
	}

	// A stale open record from yesterday must never be treated as today's clock session.
	if _, err := dataStore.pool.Exec(ctx, `
		INSERT INTO attendance_entries (
			organization_id, employee_id, work_date, check_in, work_mode, status, source
		) VALUES ($1::uuid, 'emp_002', current_date - 1, now() - interval '1 day 8 hours', 'office', 'present', 'self_service')
		ON CONFLICT (organization_id, employee_id, work_date) DO UPDATE
		SET check_in=EXCLUDED.check_in, check_out=NULL, source='self_service'`, orgID); err != nil {
		t.Fatal(err)
	}
	_, err = dataStore.CheckOutTodayV2(ctx, orgID, "emp_002")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected stale checkout guard to return not found, got %v", err)
	}
}

func mustDateAttendance(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func mustRFC3339Attendance(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
