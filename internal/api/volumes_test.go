package api

import (
	"encoding/json"
	"testing"
)

func TestListVolumes(t *testing.T) {
	client := &MockClient{
		ListVolumesFunc: func(projectID, environmentID string) ([]VolumeInstance, error) {
			if projectID != "project-1" || environmentID != "env-1" {
				t.Errorf("unexpected params: projectID=%s, environmentID=%s", projectID, environmentID)
			}

			serviceID := "service-1"
			return []VolumeInstance{
				{Volume: Volume{ID: "vol-1", Name: "my-data"},
					MountPath:     "/app/data",
					ServiceID:     &serviceID,
					CurrentSizeMB: 125.5,
					SizeMB:        5000,
				},
			}, nil
		},
	}

	volumes, err := client.ListVolumes("project-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(volumes))
	}

	if volumes[0].Volume.Name != "my-data" {
		t.Errorf("unexpected volume name: %s", volumes[0].Volume.Name)
	}
}

func TestCreateVolume(t *testing.T) {
	client := &MockClient{
		CreateVolumeFunc: func(projectID, environmentID, serviceID, mountPath string) (Volume, error) {
			if projectID != "project-1" || environmentID != "env-1" || serviceID != "service-1" || mountPath != "/app/data" {
				t.Errorf("unexpected params")
			}
			return Volume{ID: "vol-1", Name: "volume_01JMK96"}, nil
		},
	}

	vol, err := client.CreateVolume("project-1", "env-1", "service-1", "/app/data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vol.ID != "vol-1" {
		t.Errorf("unexpected volume ID: %s", vol.ID)
	}
}

func TestDeleteVolume(t *testing.T) {
	client := &MockClient{
		DeleteVolumeFunc: func(volumeID string) error {
			if volumeID != "vol-1" {
				t.Errorf("unexpected volumeID: %s", volumeID)
			}
			return nil
		},
	}

	err := client.DeleteVolume("vol-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateVolumeName(t *testing.T) {
	client := &MockClient{
		UpdateVolumeNameFunc: func(volumeID, name string) error {
			if volumeID != "vol-1" || name != "uploads" {
				t.Errorf("unexpected params: volumeID=%s, name=%s", volumeID, name)
			}
			return nil
		},
	}

	err := client.UpdateVolumeName("vol-1", "uploads")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateVolumeMountPath(t *testing.T) {
	client := &MockClient{
		UpdateVolumeMountPathFunc: func(volumeID, serviceID, environmentID, mountPath string) error {
			if volumeID != "vol-1" || serviceID != "service-1" || environmentID != "env-1" || mountPath != "/app/uploads" {
				t.Errorf("unexpected params")
			}
			return nil
		},
	}

	err := client.UpdateVolumeMountPath("vol-1", "service-1", "env-1", "/app/uploads")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAttachVolume(t *testing.T) {
	client := &MockClient{
		AttachVolumeFunc: func(volumeID, serviceID, environmentID string) error {
			if volumeID != "vol-1" || serviceID != "service-2" || environmentID != "env-1" {
				t.Errorf("unexpected params")
			}
			return nil
		},
	}

	err := client.AttachVolume("vol-1", "service-2", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDetachVolume(t *testing.T) {
	client := &MockClient{
		DetachVolumeFunc: func(volumeID, environmentID string) error {
			if volumeID != "vol-1" || environmentID != "env-1" {
				t.Errorf("unexpected params")
			}
			return nil
		},
	}

	err := client.DetachVolume("vol-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumeInstanceJSON(t *testing.T) {
	// Test JSON marshaling/unmarshaling
	serviceID := "service-1"
	original := VolumeInstance{
		Volume:        Volume{ID: "vol-1", Name: "my-data"},
		MountPath:     "/app/data",
		ServiceID:     &serviceID,
		CurrentSizeMB: 125.5,
		SizeMB:        5000,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled VolumeInstance
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Volume.Name != original.Volume.Name {
		t.Errorf("name mismatch: %s != %s", unmarshaled.Volume.Name, original.Volume.Name)
	}

	if *unmarshaled.ServiceID != *original.ServiceID {
		t.Errorf("serviceID mismatch")
	}
}
