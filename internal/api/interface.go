// Package api provides the Railway GraphQL API client.
package api

import "github.com/kubenoops/railctl/internal/types"

// RegistryCredentials holds authentication for private Docker registries.
type RegistryCredentials struct {
	Username string
	Password string
}

// APIClient defines the interface for Railway API operations.
// This interface enables dependency injection and mock testing.
type APIClient interface {
	// Projects
	ListProjects() ([]types.Project, error)
	GetProject(id string) (types.Project, error)
	CreateProject(name string) (types.Project, error)
	DeleteProject(id string) error

	// Environments
	ListEnvironments(projectID string) ([]types.Environment, error)
	CreateEnvironment(projectID, name string) (types.Environment, error)
	DeleteEnvironment(id string) error

	// Services
	ListServices(projectID, environmentID string) ([]types.ServiceDetail, error)
	GetService(id string) (types.ServiceDetail, error)
	CreateService(projectID, environmentID, name, image string, creds *RegistryCredentials) (types.Service, error)
	UpdateServiceInstance(serviceID, environmentID, image string, creds *RegistryCredentials) error
	UpdateServiceInstanceConfig(serviceID, environmentID string, startCommand, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) error
	DeployServiceInstance(serviceID, environmentID string) (string, error)
	RedeployDeployment(deploymentID string) error
	DeleteService(id string) error
	DeleteServiceInstance(serviceID, environmentID string) error
	GetBuildLogs(deploymentID string, limit int) ([]string, error)

	// Variables
	GetVariables(projectID, environmentID, serviceID string) (map[string]string, error)
	GetSharedVariables(projectID, environmentID string) (map[string]string, error)
	SetSharedVariables(projectID, environmentID string, variables map[string]string) error
	GetRawVariables(projectID, environmentID, serviceID string) (map[string]string, error)
	GetSealedVariables(environmentID, serviceID string) (map[string]bool, error)
	SetVariables(projectID, environmentID, serviceID string, variables map[string]string, skipDeploys bool) error
	DeleteVariable(projectID, environmentID, serviceID, name string) error

	// Deployments
	ListDeployments(projectID, environmentID, serviceID string, limit int) ([]Deployment, error)
	RemoveDeployment(deploymentID string) error
	GetDeploymentLogs(deploymentID string, limit int) ([]LogEntry, error)

	// SSH exec / port-forward
	GetServiceInstanceID(environmentID, serviceID string) (string, error)
	ListReplicas(environmentID, serviceID string) (ReplicaList, error)

	// Domains
	ListDomains(projectID, environmentID, serviceID string) (DomainList, error)
	CreateServiceDomain(serviceID, environmentID string, targetPort int) (ServiceDomain, error)
	UpdateServiceDomainPort(serviceDomainID, domain, environmentID, serviceID string, port int) error
	CreateCustomDomain(projectID, environmentID, serviceID, domain string, targetPort int) (CustomDomain, error)
	UpdateCustomDomainPort(customDomainID, environmentID string, port int) error
	DeleteServiceDomain(id string) error
	DeleteCustomDomain(id string) error

	// TCP Proxies
	ListTCPProxies(environmentID, serviceID string) ([]TCPProxy, error)
	CreateTCPProxy(applicationPort int, environmentID, serviceID string) (TCPProxy, error)
	DeleteTCPProxy(id string) error

	// Volumes
	ListVolumes(projectID, environmentID string) ([]VolumeInstance, error)
	CreateVolume(projectID, environmentID, serviceID, mountPath string) (Volume, error)
	DeleteVolume(volumeID string) error
	UpdateVolumeName(volumeID, name string) error
	UpdateVolumeMountPath(volumeID, serviceID, environmentID, mountPath string) error
	AttachVolume(volumeID, serviceID, environmentID string) error
	DetachVolume(volumeID, environmentID string) error

	// Volume backups
	ListVolumeBackupSchedules(volumeInstanceID string) ([]BackupSchedule, error)
	SetVolumeBackupSchedules(volumeInstanceID string, kinds []string) error
	ListVolumeBackups(volumeInstanceID string) ([]VolumeBackup, error)
	CreateVolumeBackup(volumeInstanceID, name string) (string, error)
	RestoreVolumeBackup(backupID, volumeInstanceID string) error
	DeleteVolumeBackup(backupID, volumeInstanceID string) error
	// Project tokens
	CreateProjectToken(projectID, environmentID, name string) (string, error)
	ListProjectTokens(projectID string) ([]ProjectToken, error)
	DeleteProjectToken(tokenID string) error

	// Workspace
	GetWorkspaceID() (string, error)
	TokenWorkspaces() ([]Workspace, error)

	// Token type
	IsProjectToken() (bool, error)
	IsWorkspaceToken() (bool, error)
	GetProjectContext() (projectID, environmentID string, err error)
}

// Ensure Client implements APIClient interface.
var _ APIClient = (*Client)(nil)
