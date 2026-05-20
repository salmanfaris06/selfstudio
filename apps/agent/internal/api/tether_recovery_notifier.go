package api

import (
	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/events"
)

type TetherRecoveryNotifier struct {
	Activity *activity.Store
	Broker   *events.Broker
}

func (n TetherRecoveryNotifier) TetherRecoveryUpdated(status cameras.TetherRecoveryStatus) {
	if n.Activity != nil {
		result := activity.ResultSuccess
		if status.Status == cameras.TetherRecoveryFailed || status.Status == cameras.TetherRecoveryPaused {
			result = activity.ResultFailure
		}
		n.Activity.RecordWithRefs("station.tether_recovery_updated", result, cameras.SanitizeTetherDiagnostic(status.Message), &status.StationID, nil)
	}
	if n.Broker != nil {
		if event, err := events.New("camera.tether_recovery_updated", "station", status.StationID, map[string]any{"recovery": status}); err == nil {
			n.Broker.Publish(event)
		}
	}
}
