package stations

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ValidationStatus string

const (
	ValidationStatusReady   ValidationStatus = "ready"
	ValidationStatusRunning ValidationStatus = "running"
	ValidationStatusSuccess ValidationStatus = "success"
	ValidationStatusWarning ValidationStatus = "warning"
	ValidationStatusFailed  ValidationStatus = "failed"
)

type WatchValidationRequest struct {
	TimeoutMs   int `json:"timeout_ms"`
	StabilityMs int `json:"stability_ms"`
}

type WatchValidationResult struct {
	StationID      string           `json:"station_id"`
	Status         ValidationStatus `json:"status"`
	Label          string           `json:"label"`
	Action         string           `json:"action"`
	SourcePath     *string          `json:"source_path"`
	DetectedAt     *time.Time       `json:"detected_at"`
	StableAt       *time.Time       `json:"stable_at"`
	ValidatedAt    time.Time        `json:"validated_at"`
	ValidationOnly bool             `json:"validation_only"`
}

type WatchValidator struct {
	store *Store
}

func NewWatchValidator(store *Store) WatchValidator {
	return WatchValidator{store: store}
}

func NormalizeWatchValidationRequest(request WatchValidationRequest) WatchValidationRequest {
	if request.TimeoutMs == 0 {
		request.TimeoutMs = 5000
	}
	if request.StabilityMs == 0 {
		request.StabilityMs = 500
	}
	if request.TimeoutMs < 1000 {
		request.TimeoutMs = 1000
	}
	if request.TimeoutMs > 15000 {
		request.TimeoutMs = 15000
	}
	if request.StabilityMs < 200 {
		request.StabilityMs = 200
	}
	if request.StabilityMs > 3000 {
		request.StabilityMs = 3000
	}
	return request
}

func (v WatchValidator) Run(ctx context.Context, stationID string, request WatchValidationRequest) (WatchValidationResult, error) {
	if v.store == nil {
		return WatchValidationResult{}, ErrStationNotFound
	}
	station, err := v.store.Get(stationID)
	if err != nil {
		return WatchValidationResult{}, err
	}
	return RunWatchValidationForStation(ctx, station, request, nil)
}

func RunWatchValidationForStation(ctx context.Context, station Station, request WatchValidationRequest, expectedNames map[string]struct{}) (WatchValidationResult, error) {
	request = NormalizeWatchValidationRequest(request)
	if info, err := os.Stat(station.InputFolder); err != nil || !info.IsDir() {
		return WatchValidationResult{}, ErrInputFolderUnavailable
	}
	deadline := time.Now().Add(time.Duration(request.TimeoutMs) * time.Millisecond)
	seen := map[string]struct{}{}
	for {
		if err := ctx.Err(); err != nil {
			return WatchValidationResult{}, ErrValidationCancelled
		}
		result, ok, err := scanWatchValidationOnce(ctx, station, request, seen, expectedNames, deadline)
		if err != nil {
			return WatchValidationResult{}, err
		}
		if ok {
			return result, nil
		}
		if time.Now().After(deadline) {
			return failedValidation(station.StationID, "Belum ada stable JPG terdeteksi", "Tempatkan test JPG di input folder station lalu jalankan validation lagi."), nil
		}
		if !sleepWithContext(ctx, 100*time.Millisecond) {
			return WatchValidationResult{}, ErrValidationCancelled
		}
	}
}

func (v WatchValidator) scanOnce(ctx context.Context, station Station, request WatchValidationRequest, seen map[string]struct{}, deadline time.Time) (WatchValidationResult, bool, error) {
	return scanWatchValidationOnce(ctx, station, request, seen, nil, deadline)
}

func scanWatchValidationOnce(ctx context.Context, station Station, request WatchValidationRequest, seen map[string]struct{}, expectedNames map[string]struct{}, deadline time.Time) (WatchValidationResult, bool, error) {
	entries, err := os.ReadDir(station.InputFolder)
	if err != nil {
		return WatchValidationResult{}, false, ErrInputFolderUnavailable
	}
	candidates := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isJPG(entry.Name()) {
			continue
		}
		if len(expectedNames) > 0 {
			if _, ok := expectedNames[entry.Name()]; !ok {
				continue
			}
		}
		candidates = append(candidates, entry)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name() < candidates[j].Name() })
	for _, entry := range candidates {
		path := filepath.Join(station.InputFolder, entry.Name())
		identity := normalizeFolder(path)
		if _, ok := seen[identity]; ok {
			continue
		}
		first, err := os.Stat(path)
		if err != nil || first.IsDir() || first.Size() <= 0 {
			continue
		}
		detectedAt := time.Now().UTC()
		if time.Now().Add(time.Duration(request.StabilityMs) * time.Millisecond).After(deadline) {
			continue
		}
		if !sleepWithContext(ctx, time.Duration(request.StabilityMs)*time.Millisecond) {
			return WatchValidationResult{}, false, ErrValidationCancelled
		}
		second, err := os.Stat(path)
		if err != nil || second.IsDir() || second.Size() <= 0 {
			continue
		}
		if first.Size() == second.Size() && first.ModTime().Equal(second.ModTime()) {
			stableAt := time.Now().UTC()
			cleaned := filepath.Clean(path)
			seen[identity] = struct{}{}
			return WatchValidationResult{
				StationID:      station.StationID,
				Status:         ValidationStatusSuccess,
				Label:          "Stable JPG terdeteksi",
				Action:         "Validation-only: file tidak diroute ke session atau customer output.",
				SourcePath:     &cleaned,
				DetectedAt:     &detectedAt,
				StableAt:       &stableAt,
				ValidatedAt:    stableAt,
				ValidationOnly: true,
			}, true, nil
		}
	}
	return WatchValidationResult{}, false, nil
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func failedValidation(stationID string, label string, action string) WatchValidationResult {
	return WatchValidationResult{StationID: stationID, Status: ValidationStatusFailed, Label: label, Action: action, ValidatedAt: time.Now().UTC(), ValidationOnly: true}
}

func isJPG(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".jpg" || ext == ".jpeg"
}
