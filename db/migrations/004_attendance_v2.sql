CREATE TABLE IF NOT EXISTS work_schedules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  timezone TEXT NOT NULL DEFAULT 'UTC',
  start_time TIME NOT NULL,
  end_time TIME NOT NULL,
  break_minutes INTEGER NOT NULL DEFAULT 60 CHECK (break_minutes >= 0 AND break_minutes <= 480),
  grace_minutes INTEGER NOT NULL DEFAULT 10 CHECK (grace_minutes >= 0 AND grace_minutes <= 240),
  overtime_threshold_minutes INTEGER NOT NULL DEFAULT 15 CHECK (overtime_threshold_minutes >= 0 AND overtime_threshold_minutes <= 480),
  work_days SMALLINT[] NOT NULL DEFAULT ARRAY[1,2,3,4,5]::SMALLINT[],
  is_default BOOLEAN NOT NULL DEFAULT false,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (end_time > start_time),
  CHECK (cardinality(work_days) > 0),
  CHECK (work_days <@ ARRAY[0,1,2,3,4,5,6]::SMALLINT[]),
  UNIQUE (organization_id, code)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_work_schedule_default
  ON work_schedules(organization_id)
  WHERE is_default = true;

CREATE TABLE IF NOT EXISTS work_schedule_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  employee_id TEXT NOT NULL,
  schedule_id UUID NOT NULL REFERENCES work_schedules(id) ON DELETE CASCADE,
  effective_from DATE NOT NULL DEFAULT current_date,
  effective_to DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (effective_to IS NULL OR effective_to >= effective_from),
  UNIQUE (organization_id, employee_id, schedule_id, effective_from),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id) ON DELETE CASCADE
);

ALTER TABLE attendance_entries
  ADD COLUMN IF NOT EXISTS schedule_id UUID REFERENCES work_schedules(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS scheduled_start_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS scheduled_end_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS break_minutes INTEGER NOT NULL DEFAULT 0 CHECK (break_minutes >= 0),
  ADD COLUMN IF NOT EXISTS late_minutes INTEGER NOT NULL DEFAULT 0 CHECK (late_minutes >= 0),
  ADD COLUMN IF NOT EXISTS early_leave_minutes INTEGER NOT NULL DEFAULT 0 CHECK (early_leave_minutes >= 0),
  ADD COLUMN IF NOT EXISTS worked_minutes INTEGER NOT NULL DEFAULT 0 CHECK (worked_minutes >= 0),
  ADD COLUMN IF NOT EXISTS overtime_minutes INTEGER NOT NULL DEFAULT 0 CHECK (overtime_minutes >= 0),
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'self_service'
    CHECK (source IN ('self_service', 'hr_override', 'correction', 'sync'));

CREATE TABLE IF NOT EXISTS attendance_correction_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  employee_id TEXT NOT NULL,
  attendance_entry_id UUID REFERENCES attendance_entries(id) ON DELETE SET NULL,
  work_date DATE NOT NULL,
  requested_check_in TIMESTAMPTZ,
  requested_check_out TIMESTAMPTZ,
  requested_work_mode TEXT NOT NULL DEFAULT 'office'
    CHECK (requested_work_mode IN ('office', 'remote', 'hybrid', 'field')),
  reason TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
  submitted_by UUID REFERENCES users(id) ON DELETE SET NULL,
  reviewer_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  review_note TEXT,
  reviewed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (requested_check_in IS NOT NULL OR requested_check_out IS NOT NULL),
  CHECK (requested_check_out IS NULL OR requested_check_in IS NULL OR requested_check_out >= requested_check_in),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_correction_pending_day
  ON attendance_correction_requests(organization_id, employee_id, work_date)
  WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_schedule_assignments_employee
  ON work_schedule_assignments(organization_id, employee_id, effective_from DESC);
CREATE INDEX IF NOT EXISTS idx_attendance_corrections_org_status
  ON attendance_correction_requests(organization_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attendance_entries_org_date_v2
  ON attendance_entries(organization_id, work_date, employee_id);
