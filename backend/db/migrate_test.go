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
	var organizationID, serverID string
	if err = pool.QueryRow(ctx, `INSERT INTO organizations(name,slug) VALUES('Migration','migration') RETURNING id`).Scan(&organizationID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO servers(organization_id,name,hostname,connection_status) VALUES($1,'legacy','legacy.internal','connected') RETURNING id`, organizationID).Scan(&serverID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO agent_enrollment_tokens(organization_id,server_id,token_hash,expires_at) VALUES($1,$2,'legacy-token',now()+interval '1 hour')`, organizationID, serverID); err != nil {
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
}
