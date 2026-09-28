package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type SearchResult struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Route    string `json:"route"`
	Status   string `json:"status,omitempty"`
}

type SearchOptions struct {
	IncludeMembers bool
	IncludeBudgets bool
}

func (r Repository) Search(ctx context.Context, organizationID uuid.UUID, query string, options SearchOptions) ([]SearchResult, error) {
	pattern := "%" + escapeLike(strings.TrimSpace(query)) + "%"
	rows, err := r.Pool.Query(ctx, `
WITH search_results AS (
    SELECT 'project'::text AS resource_type, p.id::text AS resource_id, p.name AS title,
           concat_ws(' · ', NULLIF(p.slug, ''), NULLIF(p.description, '')) AS subtitle,
           '/projects/' || p.id::text AS route, ''::text AS status, 10 AS type_order
    FROM projects p
    WHERE p.organization_id=$1 AND (p.name ILIKE $2 ESCAPE E'\\' OR p.slug ILIKE $2 ESCAPE E'\\' OR p.description ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'environment', e.id::text, e.name,
           concat_ws(' · ', p.name, e.slug), '/environments', '', 20
    FROM environments e
    JOIN projects p ON p.id=e.project_id AND p.organization_id=e.organization_id
    WHERE e.organization_id=$1 AND (e.name ILIKE $2 ESCAPE E'\\' OR e.slug ILIKE $2 ESCAPE E'\\' OR p.name ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'application', a.id::text, a.name,
           concat_ws(' · ', p.name || ' / ' || e.name, a.source_type, NULLIF(a.image, '')), '/applications', '', 30
    FROM applications a
    JOIN environments e ON e.id=a.environment_id AND e.organization_id=a.organization_id
    JOIN projects p ON p.id=a.project_id AND p.organization_id=a.organization_id
    WHERE a.organization_id=$1 AND (a.name ILIKE $2 ESCAPE E'\\' OR a.source_type ILIKE $2 ESCAPE E'\\' OR COALESCE(a.image, '') ILIKE $2 ESCAPE E'\\' OR e.name ILIKE $2 ESCAPE E'\\' OR p.name ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'deployment', d.id::text, a.name || ' #' || d.number::text,
           concat_ws(' · ', d.status, NULLIF(d.repository, ''), NULLIF(d.branch, ''), NULLIF(d.commit_sha, '')), '/deployments', d.status, 40
    FROM deployments d
    JOIN applications a ON a.id=d.application_id AND a.organization_id=d.organization_id
    WHERE d.organization_id=$1 AND (a.name ILIKE $2 ESCAPE E'\\' OR d.number::text ILIKE $2 ESCAPE E'\\' OR d.status ILIKE $2 ESCAPE E'\\' OR d.repository ILIKE $2 ESCAPE E'\\' OR d.branch ILIKE $2 ESCAPE E'\\' OR d.commit_sha ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'server', s.id::text, s.name,
           concat_ws(' · ', s.connection_type, NULLIF(s.hostname, ''), NULLIF(s.public_address, '')), '/servers', s.connection_status, 50
    FROM servers s
    WHERE s.organization_id=$1 AND (s.name ILIKE $2 ESCAPE E'\\' OR s.hostname ILIKE $2 ESCAPE E'\\' OR s.public_address ILIKE $2 ESCAPE E'\\' OR s.connection_type ILIKE $2 ESCAPE E'\\' OR s.connection_status ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'domain', d.id::text, d.hostname,
           concat_ws(' · ', d.routing_mode, d.protocol || ':' || d.target_port::text), '/domains', d.dns_state, 60
    FROM domains d
    WHERE d.organization_id=$1 AND (d.hostname ILIKE $2 ESCAPE E'\\' OR d.routing_mode ILIKE $2 ESCAPE E'\\' OR d.dns_state ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'aws_account', a.id::text, a.display_name,
           concat_ws(' · ', a.account_id, a.default_region, a.role_arn), '/aws/accounts', a.status, 70
    FROM aws_accounts a
    WHERE a.organization_id=$1 AND (a.display_name ILIKE $2 ESCAPE E'\\' OR a.account_id ILIKE $2 ESCAPE E'\\' OR a.default_region ILIKE $2 ESCAPE E'\\' OR a.role_arn ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'aws_instance', i.id::text, COALESCE(NULLIF(i.name, ''), i.provider_instance_id),
           concat_ws(' · ', i.provider_instance_id, i.instance_type, i.region, NULLIF(i.public_ip, ''), NULLIF(i.private_ip, '')), '/aws/compute', i.state, 80
    FROM aws_instances i
    WHERE i.organization_id=$1 AND (i.name ILIKE $2 ESCAPE E'\\' OR i.provider_instance_id ILIKE $2 ESCAPE E'\\' OR i.instance_type ILIKE $2 ESCAPE E'\\' OR i.region ILIKE $2 ESCAPE E'\\' OR i.public_ip ILIKE $2 ESCAPE E'\\' OR i.private_ip ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'budget', b.id::text, b.name,
           trim(to_char(b.monthly_amount, 'FM999999999990.00')) || ' ' || b.currency, '/aws/costs', '', 90
    FROM aws_budgets b
    WHERE $4 AND b.organization_id=$1 AND (b.name ILIKE $2 ESCAPE E'\\' OR b.currency ILIKE $2 ESCAPE E'\\')
    UNION ALL
    SELECT 'member', u.id::text, u.display_name,
           u.email || ' · ' || m.role, '/members', m.role, 100
    FROM memberships m
    JOIN users u ON u.id=m.user_id
    WHERE $3 AND m.organization_id=$1 AND (u.display_name ILIKE $2 ESCAPE E'\\' OR u.email ILIKE $2 ESCAPE E'\\' OR m.role ILIKE $2 ESCAPE E'\\')
)
SELECT resource_id,resource_type,title,subtitle,route,status
FROM search_results
ORDER BY type_order,title
LIMIT 60`, organizationID, pattern, options.IncludeMembers, options.IncludeBudgets)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []SearchResult{}
	for rows.Next() {
		var result SearchResult
		if err = rows.Scan(&result.ID, &result.Type, &result.Title, &result.Subtitle, &result.Route, &result.Status); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
