package cameras

import (
	"context"

	"selfstudio/agent/internal/stations"
)

type DiscoveryReadinessAdapter struct{ Service DiscoveryService }

func (a DiscoveryReadinessAdapter) Discover(ctx context.Context) (stations.CameraDiscoveryResult, error) {
	result, err := a.Service.Discover(ctx)
	converted := stations.CameraDiscoveryResult{Status: string(result.Status), Action: string(result.Action), Runtime: string(result.Runtime), Cameras: make([]stations.CameraDetected, 0, len(result.Cameras))}
	for _, camera := range result.Cameras {
		converted.Cameras = append(converted.Cameras, stations.CameraDetected{IdentityKey: camera.IdentityKey, Port: camera.Port, Runtime: string(camera.Runtime), Connected: camera.Connected})
	}
	return converted, err
}

type TetherReadinessAdapter struct{ Supervisor *TetherSupervisor }

func (a TetherReadinessAdapter) Status(stationID string) stations.TetherStatusResult {
	if a.Supervisor == nil {
		return stations.TetherStatusResult{Status: string(TetherStatusStopped), LastErrorAction: string(ActionStartTetherListener)}
	}
	listener := a.Supervisor.Status(stationID)
	return stations.TetherStatusResult{Status: string(listener.Status), LastErrorAction: string(listener.LastErrorAction)}
}
