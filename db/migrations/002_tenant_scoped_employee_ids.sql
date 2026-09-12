-- Existing installations created before tenant-scoped employee IDs carried a
-- redundant global UNIQUE constraint on employees.id. The composite primary key
-- (organization_id, id) is the intended identity boundary.
ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_id_key;
