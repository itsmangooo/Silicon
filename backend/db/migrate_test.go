package db

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAgentSchemaMigrationPreservesServer(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	configuration, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.ConnConfig.Database != "silicon_test" {
		t.Fatalf("migration test refuses database %q", configuration.ConnConfig.Database)
	}
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public; CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000001_initial", "000002_integrations", "000003_docker_runtime", "000004_agent"} {
		body, readErr := migrations.ReadFile("migrations/" + name + ".up.sql")
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, name); err != nil {
			t.Fatal(err)
		}
	}
	var organizationID, serverID, projectID, environmentID, applicationID, deploymentID, githubIntegrationID string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name,slug) VALUES('Migration','migration') RETURNING id`).Scan(&organizationID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO projects(organization_id,name,slug) VALUES($1,'Preserved project','preserved-project') RETURNING id`, organizationID).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO environments(organization_id,project_id,name,slug) VALUES($1,$2,'Production','production') RETURNING id`, organizationID, projectID).Scan(&environmentID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO applications(organization_id,project_id,environment_id,name,image,internal_port) VALUES($1,$2,$3,'API','registry.example/api:1',3000) RETURNING id`, organizationID, projectID, environmentID).Scan(&applicationID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO deployments(organization_id,application_id,number,source_revision,image,status) VALUES($1,$2,1,'abc123','registry.example/api:1','healthy') RETURNING id`, organizationID, applicationID).Scan(&deploymentID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO servers(organization_id,name,hostname,connection_status) VALUES($1,'legacy','legacy.internal','connected') RETURNING id`, organizationID).Scan(&serverID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO secrets(organization_id,environment_id,application_id,name,encrypted_value) VALUES($1,$2,$3,'DATABASE_PASSWORD',decode('010203','hex'))`, organizationID, environmentID, applicationID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO application_environment_variables(organization_id,environment_id,application_id,name,value) VALUES($1,$2,$3,'LOG_LEVEL','info')`, organizationID, environmentID, applicationID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO runtime_instances(organization_id,application_id,deployment_id,provider,external_id,image,state,health,container_port) VALUES($1,$2,$3,'docker','container-preserved','registry.example/api:1','running','healthy',3000)`, organizationID, applicationID, deploymentID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO domains(organization_id,environment_id,application_id,hostname,target_port) VALUES($1,$2,$3,'api.example.test',3000)`, organizationID, environmentID, applicationID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO github_integrations(organization_id,installation_id,account_login) VALUES($1,42,'migration-test') RETURNING id`, organizationID).Scan(&githubIntegrationID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO application_git_sources(application_id,organization_id,integration_id,repository_id,repository_full_name,branch,auto_deploy) VALUES($1,$2,$3,99,'example/preserved','main',true)`, applicationID, organizationID, githubIntegrationID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO cloudflare_integrations(organization_id,account_id,encrypted_api_token) VALUES($1,'account-preserved',decode('040506','hex'))`, organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO agent_enrollment_tokens(organization_id,server_id,token_hash,expires_at) VALUES($1,$2,'legacy-token',now()+interval '1 hour')`, organizationID, serverID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO users(email,display_name,password_hash,created_at) VALUES
		('first@example.test','First user','hash',now()-interval '1 day'),
		('second@example.test','Second user','hash',now())`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var connectionType, connectionStatus, connectionError, hostname string
	if err = pool.QueryRow(ctx, `SELECT connection_type,connection_status,connection_error,hostname FROM servers WHERE id=$1`, serverID).Scan(&connectionType, &connectionStatus, &connectionError, &hostname); err != nil {
		t.Fatal(err)
	}
	if connectionType != "unconfigured" || connectionStatus != "connection_not_configured" || hostname != "legacy.internal" || !strings.Contains(connectionError, "local or SSH") {
		t.Fatalf("migrated server type=%q status=%q host=%q error=%q", connectionType, connectionStatus, hostname, connectionError)
	}
	var enrollmentTable, identityTable *string
	if err = pool.QueryRow(ctx, `SELECT to_regclass('public.agent_enrollment_tokens')::text,to_regclass('public.server_agents')::text`).Scan(&enrollmentTable, &identityTable); err != nil {
		t.Fatal(err)
	}
	if enrollmentTable != nil || identityTable != nil {
		t.Fatalf("retired tables remain: enrollment=%v identity=%v", enrollmentTable, identityTable)
	}
	var preservedRecords int64
	if err = pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM projects WHERE id=$1) +
		(SELECT count(*) FROM environments WHERE id=$2) +
		(SELECT count(*) FROM applications WHERE id=$3) +
		(SELECT count(*) FROM deployments WHERE id=$4) +
		(SELECT count(*) FROM runtime_instances WHERE deployment_id=$4) +
		(SELECT count(*) FROM secrets WHERE application_id=$3) +
		(SELECT count(*) FROM application_environment_variables WHERE application_id=$3) +
		(SELECT count(*) FROM domains WHERE application_id=$3) +
		(SELECT count(*) FROM github_integrations WHERE organization_id=$5) +
		(SELECT count(*) FROM application_git_sources WHERE application_id=$3) +
		(SELECT count(*) FROM cloudflare_integrations WHERE organization_id=$5)
	`, projectID, environmentID, applicationID, deploymentID, organizationID).Scan(&preservedRecords); err != nil {
		t.Fatal(err)
	}
	if preservedRecords != 11 {
		t.Fatalf("forward migrations preserved %d representative records, want 11", preservedRecords)
	}
	var migratedSecretProjectID string
	var migratedSecretCiphertext []byte
	if err = pool.QueryRow(ctx, `SELECT project_id,encrypted_value FROM secrets WHERE application_id=$1 AND name='DATABASE_PASSWORD'`, applicationID).Scan(&migratedSecretProjectID, &migratedSecretCiphertext); err != nil {
		t.Fatal(err)
	}
	if migratedSecretProjectID != projectID || string(migratedSecretCiphertext) != string([]byte{1, 2, 3}) {
		t.Fatalf("application secret hierarchy migration project=%q ciphertext=%x", migratedSecretProjectID, migratedSecretCiphertext)
	}
	var administratorEmail string
	var administratorCount int
	if err = pool.QueryRow(ctx, `SELECT email::text FROM users WHERE is_system_admin`).Scan(&administratorEmail); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_system_admin`).Scan(&administratorCount); err != nil {
		t.Fatal(err)
	}
	if administratorEmail != "first@example.test" || administratorCount != 1 {
		t.Fatalf("migration system administrator email=%q count=%d", administratorEmail, administratorCount)
	}
}
