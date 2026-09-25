package authorization

type Permission string

const (
	ProjectRead            Permission = "project.read"
	ProjectCreate          Permission = "project.create"
	ProjectUpdate          Permission = "project.update"
	ProjectDelete          Permission = "project.delete"
	EnvironmentRead        Permission = "environment.read"
	EnvironmentCreate      Permission = "environment.create"
	ApplicationRead        Permission = "application.read"
	ApplicationCreate      Permission = "application.create"
	ApplicationUpdate      Permission = "application.update"
	ApplicationDelete      Permission = "application.delete"
	DeploymentRead         Permission = "deployment.read"
	DeploymentCreate       Permission = "deployment.create"
	DeploymentRollback     Permission = "deployment.rollback"
	ServerRead             Permission = "server.read"
	ServerManage           Permission = "server.manage"
	LogsRead               Permission = "logs.read"
	DomainManage           Permission = "domain.manage"
	SecretWrite            Permission = "secret.write"
	MemberRead             Permission = "member.read"
	MemberManage           Permission = "member.manage"
	IdentityProviderRead   Permission = "identity_provider.read"
	IdentityProviderManage Permission = "identity_provider.manage"
	AuditRead              Permission = "audit.read"
	OrganizationRead       Permission = "organization.read"
	OrganizationManage     Permission = "organization.manage"
	IntegrationRead        Permission = "integration.read"
	IntegrationManage      Permission = "integration.manage"
)

var rolePermissions = map[string]map[Permission]struct{}{
	"owner": allPermissions(),
	"admin": set(
		ProjectRead, ProjectCreate, ProjectUpdate, ProjectDelete,
		EnvironmentRead, EnvironmentCreate,
		ApplicationRead, ApplicationCreate, ApplicationUpdate, ApplicationDelete,
		DeploymentRead, DeploymentCreate, DeploymentRollback,
		ServerRead, ServerManage, LogsRead, DomainManage, SecretWrite,
		MemberRead, MemberManage, IdentityProviderRead, IdentityProviderManage,
		AuditRead, OrganizationRead, OrganizationManage,
		IntegrationRead, IntegrationManage,
	),
	"developer": set(
		ProjectRead, ProjectCreate, ProjectUpdate,
		EnvironmentRead, EnvironmentCreate,
		ApplicationRead, ApplicationCreate, ApplicationUpdate,
		DeploymentRead, DeploymentCreate, DeploymentRollback,
		ServerRead, LogsRead, DomainManage, SecretWrite,
		MemberRead, IdentityProviderRead, OrganizationRead,
		IntegrationRead,
	),
	"viewer": set(
		ProjectRead, EnvironmentRead, ApplicationRead, DeploymentRead,
		ServerRead, LogsRead, MemberRead, IdentityProviderRead,
		AuditRead, OrganizationRead,
		IntegrationRead,
	),
}

func Allowed(role string, permission Permission) bool {
	_, ok := rolePermissions[role][permission]
	return ok
}

func Permissions(role string) []Permission {
	permissions := rolePermissions[role]
	result := make([]Permission, 0, len(permissions))
	for permission := range permissions {
		result = append(result, permission)
	}
	return result
}

func set(values ...Permission) map[Permission]struct{} {
	result := make(map[Permission]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func allPermissions() map[Permission]struct{} {
	return set(
		ProjectRead, ProjectCreate, ProjectUpdate, ProjectDelete,
		EnvironmentRead, EnvironmentCreate,
		ApplicationRead, ApplicationCreate, ApplicationUpdate, ApplicationDelete,
		DeploymentRead, DeploymentCreate, DeploymentRollback,
		ServerRead, ServerManage, LogsRead, DomainManage, SecretWrite,
		MemberRead, MemberManage, IdentityProviderRead, IdentityProviderManage,
		AuditRead, OrganizationRead, OrganizationManage,
		IntegrationRead, IntegrationManage,
	)
}
