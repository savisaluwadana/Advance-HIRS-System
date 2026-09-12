CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS organizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  display_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS employees (
  id TEXT NOT NULL,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  employee_number TEXT NOT NULL,
  first_name TEXT NOT NULL,
  last_name TEXT NOT NULL,
  work_email TEXT NOT NULL,
  job_title TEXT,
  department TEXT,
  manager_id TEXT,
  employment_type TEXT NOT NULL DEFAULT 'full_time'
    CHECK (employment_type IN ('full_time', 'part_time', 'contract', 'intern', 'temporary')),
  location TEXT,
  start_date DATE,
  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'leave', 'inactive', 'onboarding')),
  server_version BIGINT NOT NULL DEFAULT 1 CHECK (server_version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, id),
  UNIQUE (organization_id, employee_number),
  UNIQUE (organization_id, work_email),
  FOREIGN KEY (organization_id, manager_id) REFERENCES employees(organization_id, id)
);

CREATE TABLE IF NOT EXISTS organization_memberships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  employee_id TEXT,
  role TEXT NOT NULL CHECK (role IN ('admin', 'hr', 'manager', 'employee')),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (organization_id, user_id),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id)
);

CREATE TABLE IF NOT EXISTS access_invitations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  email TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin', 'hr', 'manager', 'employee')),
  employee_id TEXT,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id)
);

CREATE TABLE IF NOT EXISTS leave_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  employee_id TEXT NOT NULL,
  leave_type TEXT NOT NULL
    CHECK (leave_type IN ('annual', 'sick', 'unpaid', 'remote', 'parental', 'other')),
  start_date DATE NOT NULL,
  end_date DATE NOT NULL,
  days NUMERIC(6,2) NOT NULL CHECK (days > 0),
  reason TEXT,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
  approver_id TEXT,
  decision_note TEXT,
  decided_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (end_date >= start_date),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id) ON DELETE CASCADE,
  FOREIGN KEY (organization_id, approver_id) REFERENCES employees(organization_id, id)
);

CREATE TABLE IF NOT EXISTS attendance_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  employee_id TEXT NOT NULL,
  work_date DATE NOT NULL,
  check_in TIMESTAMPTZ,
  check_out TIMESTAMPTZ,
  work_mode TEXT NOT NULL DEFAULT 'office'
    CHECK (work_mode IN ('office', 'remote', 'hybrid', 'field')),
  status TEXT NOT NULL DEFAULT 'present'
    CHECK (status IN ('present', 'absent', 'leave', 'holiday')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (organization_id, employee_id, work_date),
  CHECK (check_out IS NULL OR check_in IS NULL OR check_out >= check_in),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  department TEXT,
  location TEXT,
  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft', 'open', 'paused', 'closed')),
  openings INTEGER NOT NULL DEFAULT 1 CHECK (openings > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS leave_policies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  leave_type TEXT NOT NULL
    CHECK (leave_type IN ('annual', 'sick', 'unpaid', 'remote', 'parental', 'other')),
  annual_entitlement NUMERIC(7,2) NOT NULL DEFAULT 0 CHECK (annual_entitlement >= 0),
  carry_over_limit NUMERIC(7,2) NOT NULL DEFAULT 0 CHECK (carry_over_limit >= 0),
  track_balance BOOLEAN NOT NULL DEFAULT true,
  allow_negative BOOLEAN NOT NULL DEFAULT false,
  requires_approval BOOLEAN NOT NULL DEFAULT true,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (organization_id, code),
  UNIQUE (organization_id, leave_type)
);

CREATE TABLE IF NOT EXISTS leave_policy_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  employee_id TEXT NOT NULL,
  policy_id UUID NOT NULL REFERENCES leave_policies(id) ON DELETE CASCADE,
  effective_from DATE NOT NULL DEFAULT current_date,
  effective_to DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (effective_to IS NULL OR effective_to >= effective_from),
  UNIQUE (organization_id, employee_id, policy_id, effective_from),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS company_holidays (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  holiday_date DATE NOT NULL,
  name TEXT NOT NULL,
  location TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS leave_balance_ledger (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  employee_id TEXT NOT NULL,
  policy_id UUID NOT NULL REFERENCES leave_policies(id) ON DELETE CASCADE,
  leave_request_id UUID REFERENCES leave_requests(id) ON DELETE SET NULL,
  balance_year INTEGER NOT NULL CHECK (balance_year BETWEEN 2000 AND 2200),
  amount NUMERIC(8,2) NOT NULL CHECK (amount <> 0),
  event_type TEXT NOT NULL
    CHECK (event_type IN ('entitlement', 'carry_over', 'adjustment', 'approved_leave', 'cancellation')),
  source_key TEXT NOT NULL,
  note TEXT,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (organization_id, employee_id, policy_id, source_key),
  FOREIGN KEY (organization_id, employee_id) REFERENCES employees(organization_id, id) ON DELETE CASCADE
);

ALTER TABLE leave_requests
  ADD COLUMN IF NOT EXISTS policy_id UUID REFERENCES leave_policies(id) ON DELETE RESTRICT,
  ADD COLUMN IF NOT EXISTS partial_day TEXT NOT NULL DEFAULT 'full'
    CHECK (partial_day IN ('full', 'first_half', 'second_half')),
  ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS cancellation_note TEXT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'leave_requests_no_overlap'
      AND conrelid = 'leave_requests'::regclass
  ) THEN
    ALTER TABLE leave_requests
      ADD CONSTRAINT leave_requests_no_overlap
      EXCLUDE USING gist (
        organization_id WITH =,
        employee_id WITH =,
        daterange(start_date, end_date, '[]') WITH &&
      ) WHERE (status IN ('pending', 'approved'));
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_invitation_pending_email
  ON access_invitations(organization_id, lower(email))
  WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_company_holidays_unique
  ON company_holidays(organization_id, holiday_date, COALESCE(location, ''));
CREATE INDEX IF NOT EXISTS idx_invitations_org_created ON access_invitations(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memberships_user ON organization_memberships(user_id, status);
CREATE INDEX IF NOT EXISTS idx_employees_org_status ON employees(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_employees_org_department ON employees(organization_id, department);
CREATE INDEX IF NOT EXISTS idx_employees_org_manager ON employees(organization_id, manager_id);
CREATE INDEX IF NOT EXISTS idx_leave_org_status ON leave_requests(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_leave_employee_created ON leave_requests(organization_id, employee_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_leave_policies_org_active ON leave_policies(organization_id, active, leave_type);
CREATE INDEX IF NOT EXISTS idx_leave_assignments_employee ON leave_policy_assignments(organization_id, employee_id, effective_from DESC);
CREATE INDEX IF NOT EXISTS idx_leave_ledger_employee_year ON leave_balance_ledger(organization_id, employee_id, balance_year, policy_id, created_at);
CREATE INDEX IF NOT EXISTS idx_company_holidays_org_date ON company_holidays(organization_id, holiday_date);
CREATE INDEX IF NOT EXISTS idx_attendance_org_date ON attendance_entries(organization_id, work_date);
CREATE INDEX IF NOT EXISTS idx_attendance_employee_date ON attendance_entries(organization_id, employee_id, work_date DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_org_status ON jobs(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_audit_org_created ON audit_events(organization_id, created_at DESC);
