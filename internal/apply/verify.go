package apply

import (
	"encoding/json"
	"time"

	"github.com/kubenoops/railctl/internal/api"
)

// Read-your-writes bounds: how long to poll for staged writes to become
// readable before the caller falls back to the fixed settle. Vars, not
// consts, so tests can shrink them.
var (
	configCommitTimeout = 30 * time.Second
	configCommitPoll    = 2 * time.Second
)

// ConfigExpectations names the writes a flow staged for one service and wants
// to read back before acting further. Empty fields are unmanaged (not
// checked): the point is to confirm what WE wrote has landed, not to model the
// whole config.
type ConfigExpectations struct {
	ServiceID string
	// RegionID is the expected deploy.multiRegionConfig key ("" = unmanaged).
	RegionID string
	// SourceImage is the expected services.<id>.source.image ("" = unmanaged).
	SourceImage string
	// VolumeID/MountPath are the expected volumeMounts entry ("" = unmanaged).
	VolumeID  string
	MountPath string
	// Variables must read back with exactly these values (nil = unmanaged).
	Variables map[string]string
	// RemovedVariables must read back ABSENT (nil = unmanaged).
	RemovedVariables []string
}

// environmentConfigView is the slice of the environment config the
// verification inspects. Field values beyond presence are not needed —
// multiRegionConfig entries are kept raw.
type environmentConfigView struct {
	Services map[string]struct {
		Source struct {
			Image string `json:"image"`
		} `json:"source"`
		Deploy struct {
			MultiRegionConfig map[string]json.RawMessage `json:"multiRegionConfig"`
		} `json:"deploy"`
		VolumeMounts map[string]struct {
			MountPath string `json:"mountPath"`
		} `json:"volumeMounts"`
	} `json:"services"`
}

// AwaitConfigCommitted polls until every expectation reads back, returning
// true the moment they do — the typical case settles in a few seconds, far
// under any fixed sleep. Railway commits config asynchronously and a rollout
// triggered while a commit is in flight gets superseded (REMOVED) when the
// reconciler applies it, which is exactly what waiting for read-your-writes
// prevents. Returns false on deadline; the caller then falls back to the
// fixed settle and proceeds — an unverified commit is better waited for than
// raced, and --await's supersede-following stays the net.
func AwaitConfigCommitted(client api.APIClient, projectID, environmentID string, exp ConfigExpectations) bool {
	deadline := time.Now().Add(configCommitTimeout)
	for {
		if configCommitted(client, projectID, environmentID, exp) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(configCommitPoll)
	}
}

// configCommitted checks every managed expectation in one pass. Any read
// error or absent value is simply not-yet-committed — never a hard failure,
// the caller's timeout decides that.
func configCommitted(client api.APIClient, projectID, environmentID string, exp ConfigExpectations) bool {
	if exp.RegionID != "" || exp.VolumeID != "" || exp.SourceImage != "" {
		raw, err := client.GetEnvironmentConfig(environmentID)
		if err != nil {
			return false
		}
		var cfg environmentConfigView
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return false
		}
		svc, ok := cfg.Services[exp.ServiceID]
		if !ok {
			return false
		}
		if exp.RegionID != "" {
			if _, ok := svc.Deploy.MultiRegionConfig[exp.RegionID]; !ok {
				return false
			}
		}
		if exp.SourceImage != "" && svc.Source.Image != exp.SourceImage {
			return false
		}
		if exp.VolumeID != "" {
			mount, ok := svc.VolumeMounts[exp.VolumeID]
			if !ok || (exp.MountPath != "" && mount.MountPath != exp.MountPath) {
				return false
			}
		}
	}
	if len(exp.Variables) > 0 || len(exp.RemovedVariables) > 0 {
		vars, err := client.GetVariables(projectID, environmentID, exp.ServiceID)
		if err != nil {
			return false
		}
		for k, v := range exp.Variables {
			if got, ok := vars[k]; !ok || got != v {
				return false
			}
		}
		for _, k := range exp.RemovedVariables {
			if _, still := vars[k]; still {
				return false
			}
		}
	}
	return true
}
