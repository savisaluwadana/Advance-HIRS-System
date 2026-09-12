CREATE EXTENSION IF NOT EXISTS btree_gist;

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
  is_default BOOLEAN NOT NULL DEFAULT false,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (organization_id, code)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_leave_policy_default_type
  ON leave_policies(organization_id, leave_type)
  WHERE is_default = true;

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

CREATE UNIQUE INDEX IF NOT EXISTS idx_company_holidays_unique
  ON company_holidays(organization_id, holiday_date, COALESCE(location, ''));

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

CREATE INDEX IF NOT EXISTS idx_leave_policies_org_active
  ON leave_policies(organization_id, active, leave_type);
CREATE INDEX IF NOT EXISTS idx_leave_assignments_employee
  ON leave_policy_assignments(organization_id, employee_id, effective_from DESC);
CREATE INDEX IF NOT EXISTS idx_leave_ledger_employee_year
  ON leave_balance_ledger(organization_id, employee_id, balance_year, policy_id, created_at);
CREATE INDEX IF NOT EXISTS idx_company_holidays_org_date
  ON company_holidays(organization_id, holiday_date);
