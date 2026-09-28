-- Enforce tenant identity across relationships whose parent IDs are globally
-- unique but whose organization columns previously remained independently
-- writable.

ALTER TABLE environments
    ADD CONSTRAINT environments_project_organization_fk
    FOREIGN KEY (project_id, organization_id)
    REFERENCES projects (id, organization_id)
    ON DELETE CASCADE;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_id_organization_unique
    UNIQUE (id, organization_id);

-- Older releases allowed this link to be written without a tenant-qualified
-- foreign key. Preserve both records while retiring any invalid association.
UPDATE aws_operations AS operation
SET job_id = NULL
FROM jobs AS job
WHERE operation.job_id = job.id
  AND operation.organization_id <> job.organization_id;

ALTER TABLE aws_operations
    DROP CONSTRAINT aws_operations_job_id_fkey;

ALTER TABLE aws_operations
    ADD CONSTRAINT aws_operations_job_organization_fk
    FOREIGN KEY (job_id, organization_id)
    REFERENCES jobs (id, organization_id)
    ON DELETE SET NULL (job_id);

-- A verified GitHub installation is globally unique, so existing deliveries
-- can be assigned to its owning tenant without changing their idempotency key.
-- Historical requests for unknown installations remain ignored intake records;
-- they never referenced or created tenant resources.
ALTER TABLE github_webhook_deliveries
    ADD COLUMN organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE;

UPDATE github_webhook_deliveries AS delivery
SET organization_id = integration.organization_id
FROM github_integrations AS integration
WHERE delivery.installation_id = integration.installation_id;

UPDATE github_webhook_deliveries
SET status = 'ignored',
    processed_at = COALESCE(processed_at, created_at)
WHERE organization_id IS NULL;

ALTER TABLE github_webhook_deliveries
    ADD CONSTRAINT github_webhook_delivery_tenant_check
    CHECK (organization_id IS NOT NULL OR status IN ('ignored', 'failed'));

CREATE INDEX github_webhook_deliveries_organization_idx
    ON github_webhook_deliveries(organization_id, created_at DESC)
    WHERE organization_id IS NOT NULL;
