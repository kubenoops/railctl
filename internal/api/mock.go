package api

import "github.com/kubenoops/railctl/internal/types"

// MockClient is a mock implementation of APIClient for testing.
type MockClient struct {
	// Projects
	ListProjectsFunc  func() ([]types.Project, error)
	GetProjectFunc    func(id string) (types.Project, error)
	CreateProjectFunc func(name string) (types.Project, error)
	DeleteProjectFunc func(id string) error

	// Environments
	ListEnvironmentsFunc  func(projectID string) ([]types.Environment, error)
	CreateEnvironmentFunc func(projectID, name string) (types.Environment, error)
	DeleteEnvironmentFunc func(id string) error

	// Services
	ListServicesFunc                func(projectID, environmentID string) ([]types.ServiceDetail, error)
	GetServiceFunc                  func(id string) (types.ServiceDetail, error)
	CreateServiceFunc               func(projectID, environmentID, name, image string, creds *RegistryCredentials) (types.Service, error)
	UpdateServiceInstanceFunc       func(serviceID, environmentID, image string, creds *RegistryCredentials) error
	UpdateServiceInstanceConfigFunc func(serviceID, environmentID string, startCommand, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) error
	DeployServiceInstanceFunc       func(serviceID, environmentID string) (string, error)
	RedeployDeploymentFunc          func(deploymentID string) error
	DeleteServiceFunc               func(id string) error
	DeleteServiceInstanceFunc       func(serviceID, environmentID string) error
	GetBuildLogsFunc                func(deploymentID string, limit int) ([]string, error)

	// Variables
	GetVariablesFunc       func(projectID, environmentID, serviceID string) (map[string]string, error)
	GetSharedVariablesFunc func(projectID, environmentID string) (map[string]string, error)
	GetRawVariablesFunc    func(projectID, environmentID, serviceID string) (map[string]string, error)
	GetSealedVariablesFunc func(environmentID, serviceID string) (map[string]bool, error)
	SetVariablesFunc       func(projectID, environmentID, serviceID string, variables map[string]string, skipDeploys bool) error
	DeleteVariableFunc     func(projectID, environmentID, serviceID, name string) error

	// Deployments
	ListDeploymentsFunc   func(projectID, environmentID, serviceID string, limit int) ([]Deployment, error)
	RemoveDeploymentFunc  func(deploymentID string) error
	GetDeploymentLogsFunc func(deploymentID string, limit int) ([]LogEntry, error)

	// Domains
	ListDomainsFunc             func(projectID, environmentID, serviceID string) (DomainList, error)
	CreateServiceDomainFunc     func(serviceID, environmentID string, targetPort int) (ServiceDomain, error)
	UpdateServiceDomainPortFunc func(serviceDomainID, domain, environmentID, serviceID string, port int) error
	CreateCustomDomainFunc      func(projectID, environmentID, serviceID, domain string, targetPort int) (CustomDomain, error)
	UpdateCustomDomainPortFunc  func(customDomainID, environmentID string, port int) error
	DeleteServiceDomainFunc     func(id string) error
	DeleteCustomDomainFunc      func(id string) error

	// TCP Proxies
	ListTCPProxiesFunc func(environmentID, serviceID string) ([]TCPProxy, error)
	CreateTCPProxyFunc func(applicationPort int, environmentID, serviceID string) (TCPProxy, error)
	DeleteTCPProxyFunc func(id string) error
	// Volumes
	ListVolumesFunc           func(projectID, environmentID string) ([]VolumeInstance, error)
	CreateVolumeFunc          func(projectID, environmentID, serviceID, mountPath string) (Volume, error)
	DeleteVolumeFunc          func(volumeID string) error
	UpdateVolumeNameFunc      func(volumeID, name string) error
	UpdateVolumeMountPathFunc func(volumeID, serviceID, environmentID, mountPath string) error
	AttachVolumeFunc          func(volumeID, serviceID, environmentID string) error
	DetachVolumeFunc          func(volumeID, environmentID string) error

	// Volume backups
	ListVolumeBackupSchedulesFunc func(volumeInstanceID string) ([]BackupSchedule, error)
	SetVolumeBackupSchedulesFunc  func(volumeInstanceID string, kinds []string) error
	ListVolumeBackupsFunc         func(volumeInstanceID string) ([]VolumeBackup, error)
	CreateVolumeBackupFunc        func(volumeInstanceID, name string) (string, error)
	RestoreVolumeBackupFunc       func(backupID, volumeInstanceID string) error
	DeleteVolumeBackupFunc        func(backupID, volumeInstanceID string) error
	// Project tokens
	CreateProjectTokenFunc func(projectID, environmentID, name string) (string, error)
	ListProjectTokensFunc  func(projectID string) ([]ProjectToken, error)
	DeleteProjectTokenFunc func(tokenID string) error

	// Workspace
	GetWorkspaceIDFunc  func() (string, error)
	TokenWorkspacesFunc func() ([]Workspace, error)

	// Token type
	IsProjectTokenFunc    func() (bool, error)
	IsWorkspaceTokenFunc  func() (bool, error)
	GetProjectContextFunc func() (string, string, error)
}

// Ensure MockClient implements APIClient.
var _ APIClient = (*MockClient)(nil)

func (m *MockClient) ListProjects() ([]types.Project, error) {
	if m.ListProjectsFunc != nil {
		return m.ListProjectsFunc()
	}
	return nil, nil
}

func (m *MockClient) GetProject(id string) (types.Project, error) {
	if m.GetProjectFunc != nil {
		return m.GetProjectFunc(id)
	}
	return types.Project{}, nil
}

func (m *MockClient) CreateProject(name string) (types.Project, error) {
	if m.CreateProjectFunc != nil {
		return m.CreateProjectFunc(name)
	}
	return types.Project{}, nil
}

func (m *MockClient) DeleteProject(id string) error {
	if m.DeleteProjectFunc != nil {
		return m.DeleteProjectFunc(id)
	}
	return nil
}

func (m *MockClient) ListEnvironments(projectID string) ([]types.Environment, error) {
	if m.ListEnvironmentsFunc != nil {
		return m.ListEnvironmentsFunc(projectID)
	}
	return nil, nil
}

func (m *MockClient) CreateEnvironment(projectID, name string) (types.Environment, error) {
	if m.CreateEnvironmentFunc != nil {
		return m.CreateEnvironmentFunc(projectID, name)
	}
	return types.Environment{}, nil
}

func (m *MockClient) DeleteEnvironment(id string) error {
	if m.DeleteEnvironmentFunc != nil {
		return m.DeleteEnvironmentFunc(id)
	}
	return nil
}

func (m *MockClient) ListServices(projectID, environmentID string) ([]types.ServiceDetail, error) {
	if m.ListServicesFunc != nil {
		return m.ListServicesFunc(projectID, environmentID)
	}
	return nil, nil
}

func (m *MockClient) GetService(id string) (types.ServiceDetail, error) {
	if m.GetServiceFunc != nil {
		return m.GetServiceFunc(id)
	}
	return types.ServiceDetail{}, nil
}

func (m *MockClient) CreateService(projectID, environmentID, name, image string, creds *RegistryCredentials) (types.Service, error) {
	if m.CreateServiceFunc != nil {
		return m.CreateServiceFunc(projectID, environmentID, name, image, creds)
	}
	return types.Service{}, nil
}

func (m *MockClient) UpdateServiceInstance(serviceID, environmentID, image string, creds *RegistryCredentials) error {
	if m.UpdateServiceInstanceFunc != nil {
		return m.UpdateServiceInstanceFunc(serviceID, environmentID, image, creds)
	}
	return nil
}

func (m *MockClient) UpdateServiceInstanceConfig(
	serviceID string,
	environmentID string,
	startCommand *string,
	restartPolicy *string,
	maxRetries *int,
	replicas *int,
	healthcheckPath *string,
	healthcheckTimeout *int,
) error {
	if m.UpdateServiceInstanceConfigFunc != nil {
		return m.UpdateServiceInstanceConfigFunc(serviceID, environmentID, startCommand, restartPolicy, maxRetries, replicas, healthcheckPath, healthcheckTimeout)
	}
	return nil
}

func (m *MockClient) DeployServiceInstance(serviceID, environmentID string) (string, error) {
	if m.DeployServiceInstanceFunc != nil {
		return m.DeployServiceInstanceFunc(serviceID, environmentID)
	}
	return "mock-deployment-id", nil
}

func (m *MockClient) RedeployDeployment(deploymentID string) error {
	if m.RedeployDeploymentFunc != nil {
		return m.RedeployDeploymentFunc(deploymentID)
	}
	return nil
}

func (m *MockClient) DeleteService(id string) error {
	if m.DeleteServiceFunc != nil {
		return m.DeleteServiceFunc(id)
	}
	return nil
}

func (m *MockClient) DeleteServiceInstance(serviceID, environmentID string) error {
	if m.DeleteServiceInstanceFunc != nil {
		return m.DeleteServiceInstanceFunc(serviceID, environmentID)
	}
	return nil
}

func (m *MockClient) GetWorkspaceID() (string, error) {
	if m.GetWorkspaceIDFunc != nil {
		return m.GetWorkspaceIDFunc()
	}
	return "mock-workspace-id", nil
}

func (m *MockClient) TokenWorkspaces() ([]Workspace, error) {
	if m.TokenWorkspacesFunc != nil {
		return m.TokenWorkspacesFunc()
	}
	return nil, nil
}

func (m *MockClient) IsProjectToken() (bool, error) {
	if m.IsProjectTokenFunc != nil {
		return m.IsProjectTokenFunc()
	}
	return false, nil
}

func (m *MockClient) IsWorkspaceToken() (bool, error) {
	if m.IsWorkspaceTokenFunc != nil {
		return m.IsWorkspaceTokenFunc()
	}
	return false, nil
}

func (m *MockClient) GetProjectContext() (string, string, error) {
	if m.GetProjectContextFunc != nil {
		return m.GetProjectContextFunc()
	}
	return "", "", nil
}

func (m *MockClient) GetBuildLogs(deploymentID string, limit int) ([]string, error) {
	if m.GetBuildLogsFunc != nil {
		return m.GetBuildLogsFunc(deploymentID, limit)
	}
	return nil, nil
}

func (m *MockClient) GetVariables(projectID, environmentID, serviceID string) (map[string]string, error) {
	if m.GetVariablesFunc != nil {
		return m.GetVariablesFunc(projectID, environmentID, serviceID)
	}
	return make(map[string]string), nil
}

func (m *MockClient) GetSharedVariables(projectID, environmentID string) (map[string]string, error) {
	if m.GetSharedVariablesFunc != nil {
		return m.GetSharedVariablesFunc(projectID, environmentID)
	}
	return make(map[string]string), nil
}

func (m *MockClient) GetRawVariables(projectID, environmentID, serviceID string) (map[string]string, error) {
	if m.GetRawVariablesFunc != nil {
		return m.GetRawVariablesFunc(projectID, environmentID, serviceID)
	}
	return make(map[string]string), nil
}

func (m *MockClient) SetVariables(projectID, environmentID, serviceID string, variables map[string]string, skipDeploys bool) error {
	if m.SetVariablesFunc != nil {
		return m.SetVariablesFunc(projectID, environmentID, serviceID, variables, skipDeploys)
	}
	return nil
}

func (m *MockClient) DeleteVariable(projectID, environmentID, serviceID, name string) error {
	if m.DeleteVariableFunc != nil {
		return m.DeleteVariableFunc(projectID, environmentID, serviceID, name)
	}
	return nil
}

func (m *MockClient) GetSealedVariables(environmentID, serviceID string) (map[string]bool, error) {
	if m.GetSealedVariablesFunc != nil {
		return m.GetSealedVariablesFunc(environmentID, serviceID)
	}
	return make(map[string]bool), nil
}

func (m *MockClient) ListDeployments(projectID, environmentID, serviceID string, limit int) ([]Deployment, error) {
	if m.ListDeploymentsFunc != nil {
		return m.ListDeploymentsFunc(projectID, environmentID, serviceID, limit)
	}
	return nil, nil
}

func (m *MockClient) RemoveDeployment(deploymentID string) error {
	if m.RemoveDeploymentFunc != nil {
		return m.RemoveDeploymentFunc(deploymentID)
	}
	return nil
}

func (m *MockClient) GetDeploymentLogs(deploymentID string, limit int) ([]LogEntry, error) {
	if m.GetDeploymentLogsFunc != nil {
		return m.GetDeploymentLogsFunc(deploymentID, limit)
	}
	return nil, nil
}

func (m *MockClient) ListVolumes(projectID, environmentID string) ([]VolumeInstance, error) {
	if m.ListVolumesFunc != nil {
		return m.ListVolumesFunc(projectID, environmentID)
	}
	return nil, nil
}

func (m *MockClient) CreateVolume(projectID, environmentID, serviceID, mountPath string) (Volume, error) {
	if m.CreateVolumeFunc != nil {
		return m.CreateVolumeFunc(projectID, environmentID, serviceID, mountPath)
	}
	return Volume{}, nil
}

func (m *MockClient) DeleteVolume(volumeID string) error {
	if m.DeleteVolumeFunc != nil {
		return m.DeleteVolumeFunc(volumeID)
	}
	return nil
}

func (m *MockClient) UpdateVolumeName(volumeID, name string) error {
	if m.UpdateVolumeNameFunc != nil {
		return m.UpdateVolumeNameFunc(volumeID, name)
	}
	return nil
}

func (m *MockClient) UpdateVolumeMountPath(volumeID, serviceID, environmentID, mountPath string) error {
	if m.UpdateVolumeMountPathFunc != nil {
		return m.UpdateVolumeMountPathFunc(volumeID, serviceID, environmentID, mountPath)
	}
	return nil
}

func (m *MockClient) AttachVolume(volumeID, serviceID, environmentID string) error {
	if m.AttachVolumeFunc != nil {
		return m.AttachVolumeFunc(volumeID, serviceID, environmentID)
	}
	return nil
}

func (m *MockClient) DetachVolume(volumeID, environmentID string) error {
	if m.DetachVolumeFunc != nil {
		return m.DetachVolumeFunc(volumeID, environmentID)
	}
	return nil
}

func (m *MockClient) ListVolumeBackupSchedules(volumeInstanceID string) ([]BackupSchedule, error) {
	if m.ListVolumeBackupSchedulesFunc != nil {
		return m.ListVolumeBackupSchedulesFunc(volumeInstanceID)
	}
	return nil, nil
}

func (m *MockClient) SetVolumeBackupSchedules(volumeInstanceID string, kinds []string) error {
	if m.SetVolumeBackupSchedulesFunc != nil {
		return m.SetVolumeBackupSchedulesFunc(volumeInstanceID, kinds)
	}
	return nil
}

func (m *MockClient) ListVolumeBackups(volumeInstanceID string) ([]VolumeBackup, error) {
	if m.ListVolumeBackupsFunc != nil {
		return m.ListVolumeBackupsFunc(volumeInstanceID)
	}
	return nil, nil
}

func (m *MockClient) CreateVolumeBackup(volumeInstanceID, name string) (string, error) {
	if m.CreateVolumeBackupFunc != nil {
		return m.CreateVolumeBackupFunc(volumeInstanceID, name)
	}
	return "", nil
}

func (m *MockClient) RestoreVolumeBackup(backupID, volumeInstanceID string) error {
	if m.RestoreVolumeBackupFunc != nil {
		return m.RestoreVolumeBackupFunc(backupID, volumeInstanceID)
	}
	return nil
}

func (m *MockClient) DeleteVolumeBackup(backupID, volumeInstanceID string) error {
	if m.DeleteVolumeBackupFunc != nil {
		return m.DeleteVolumeBackupFunc(backupID, volumeInstanceID)
	}
	return nil
}

func (m *MockClient) ListDomains(projectID, environmentID, serviceID string) (DomainList, error) {
	if m.ListDomainsFunc != nil {
		return m.ListDomainsFunc(projectID, environmentID, serviceID)
	}
	return DomainList{}, nil
}

func (m *MockClient) CreateServiceDomain(serviceID, environmentID string, targetPort int) (ServiceDomain, error) {
	if m.CreateServiceDomainFunc != nil {
		return m.CreateServiceDomainFunc(serviceID, environmentID, targetPort)
	}
	return ServiceDomain{}, nil
}

func (m *MockClient) UpdateServiceDomainPort(serviceDomainID, domain, environmentID, serviceID string, port int) error {
	if m.UpdateServiceDomainPortFunc != nil {
		return m.UpdateServiceDomainPortFunc(serviceDomainID, domain, environmentID, serviceID, port)
	}
	return nil
}

func (m *MockClient) CreateCustomDomain(projectID, environmentID, serviceID, domain string, targetPort int) (CustomDomain, error) {
	if m.CreateCustomDomainFunc != nil {
		return m.CreateCustomDomainFunc(projectID, environmentID, serviceID, domain, targetPort)
	}
	return CustomDomain{}, nil
}

func (m *MockClient) UpdateCustomDomainPort(customDomainID, environmentID string, port int) error {
	if m.UpdateCustomDomainPortFunc != nil {
		return m.UpdateCustomDomainPortFunc(customDomainID, environmentID, port)
	}
	return nil
}

func (m *MockClient) DeleteServiceDomain(id string) error {
	if m.DeleteServiceDomainFunc != nil {
		return m.DeleteServiceDomainFunc(id)
	}
	return nil
}

func (m *MockClient) DeleteCustomDomain(id string) error {
	if m.DeleteCustomDomainFunc != nil {
		return m.DeleteCustomDomainFunc(id)
	}
	return nil
}

func (m *MockClient) ListTCPProxies(environmentID, serviceID string) ([]TCPProxy, error) {
	if m.ListTCPProxiesFunc != nil {
		return m.ListTCPProxiesFunc(environmentID, serviceID)
	}
	return nil, nil
}

func (m *MockClient) CreateTCPProxy(applicationPort int, environmentID, serviceID string) (TCPProxy, error) {
	if m.CreateTCPProxyFunc != nil {
		return m.CreateTCPProxyFunc(applicationPort, environmentID, serviceID)
	}
	return TCPProxy{}, nil
}

func (m *MockClient) DeleteTCPProxy(id string) error {
	if m.DeleteTCPProxyFunc != nil {
		return m.DeleteTCPProxyFunc(id)
	}
	return nil
}

func (m *MockClient) CreateProjectToken(projectID, environmentID, name string) (string, error) {
	if m.CreateProjectTokenFunc != nil {
		return m.CreateProjectTokenFunc(projectID, environmentID, name)
	}
	return "", nil
}

func (m *MockClient) ListProjectTokens(projectID string) ([]ProjectToken, error) {
	if m.ListProjectTokensFunc != nil {
		return m.ListProjectTokensFunc(projectID)
	}
	return nil, nil
}

func (m *MockClient) DeleteProjectToken(tokenID string) error {
	if m.DeleteProjectTokenFunc != nil {
		return m.DeleteProjectTokenFunc(tokenID)
	}
	return nil
}
