// Package types defines domain models for Railway resources.
package types

import "time"

// Project represents a Railway project with its associated resources.
type Project struct {
	ID           string        `json:"id" yaml:"id"`
	Name         string        `json:"name" yaml:"name"`
	UpdatedAt    time.Time     `json:"updatedAt" yaml:"updatedAt"`
	Environments []Environment `json:"environments" yaml:"environments"`
	Services     []Service     `json:"services" yaml:"services"`
}

// Environment represents a Railway environment within a project.
type Environment struct {
	ID           string    `json:"id" yaml:"id"`
	Name         string    `json:"name" yaml:"name"`
	ServiceCount int       `json:"serviceCount,omitempty" yaml:"serviceCount,omitempty"`
	Services     []Service `json:"services,omitempty" yaml:"services,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

// Service represents a Railway service within a project.
type Service struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

// ServiceDomain represents a Railway-generated service domain exposed by the CLI.
type ServiceDomain struct {
	ID         string `json:"id" yaml:"id"`
	Domain     string `json:"domain" yaml:"domain"`
	TargetPort *int   `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
}

// CustomDomain represents a user-configured custom domain exposed by the CLI.
type CustomDomain struct {
	ID         string `json:"id" yaml:"id"`
	Domain     string `json:"domain" yaml:"domain"`
	TargetPort *int   `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
}

// TCPProxy represents a Railway TCP proxy exposed by the CLI.
type TCPProxy struct {
	ID              string `json:"id" yaml:"id"`
	Domain          string `json:"domain" yaml:"domain"`
	ProxyPort       int    `json:"proxyPort" yaml:"proxyPort"`
	ApplicationPort int    `json:"applicationPort" yaml:"applicationPort"`
}

// ServiceDetail represents detailed service information.
type ServiceDetail struct {
	ID                 string          `json:"id" yaml:"id"`
	Name               string          `json:"name" yaml:"name"`
	Icon               string          `json:"icon,omitempty" yaml:"icon,omitempty"`
	Source             string          `json:"source,omitempty" yaml:"source,omitempty"`
	SourceType         string          `json:"sourceType,omitempty" yaml:"sourceType,omitempty"`
	InstanceID         string          `json:"instanceId,omitempty" yaml:"instanceId,omitempty"`
	StartCommand       string          `json:"startCommand,omitempty" yaml:"startCommand,omitempty"`
	RestartPolicy      string          `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	MaxRetries         int             `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	Replicas           int             `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	HealthcheckPath    string          `json:"healthcheckPath,omitempty" yaml:"healthcheckPath,omitempty"`
	HealthcheckTimeout int             `json:"healthcheckTimeout,omitempty" yaml:"healthcheckTimeout,omitempty"`
	UpdatedAt          time.Time       `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Status             string          `json:"status,omitempty" yaml:"status,omitempty"`
	DeploymentID       string          `json:"deploymentId,omitempty" yaml:"deploymentId,omitempty"`
	DeployedAt         time.Time       `json:"deployedAt,omitempty" yaml:"deployedAt,omitempty"`
	DeploymentError    string          `json:"deploymentError,omitempty" yaml:"deploymentError,omitempty"`
	ServiceDomains     []ServiceDomain `json:"serviceDomains,omitempty" yaml:"serviceDomains,omitempty"`
	CustomDomains      []CustomDomain  `json:"customDomains,omitempty" yaml:"customDomains,omitempty"`
	TCPProxies         []TCPProxy      `json:"tcpProxies,omitempty" yaml:"tcpProxies,omitempty"`
}

// EnvironmentCount returns the number of environments in the project.
func (p Project) EnvironmentCount() int {
	return len(p.Environments)
}

// ServiceCount returns the number of services in the project.
func (p Project) ServiceCount() int {
	return len(p.Services)
}

// EnvironmentNames returns a comma-separated list of environment names.
func (p Project) EnvironmentNames() string {
	if len(p.Environments) == 0 {
		return ""
	}
	names := ""
	for i, env := range p.Environments {
		if i > 0 {
			names += ", "
		}
		names += env.Name
	}
	return names
}

// ServiceNames returns a comma-separated list of service names.
func (p Project) ServiceNames() string {
	if len(p.Services) == 0 {
		return ""
	}
	names := ""
	for i, svc := range p.Services {
		if i > 0 {
			names += ", "
		}
		names += svc.Name
	}
	return names
}
