package readiness

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"selfstudio/agent/internal/stations"
)

var ErrUnavailable = errors.New("event readiness unavailable")

type Status string

const (
	StatusReady       Status = "ready"
	StatusWarning     Status = "warning"
	StatusFailed      Status = "failed"
	StatusUnknown     Status = "unknown"
	StatusPlaceholder Status = "placeholder"
)

type Category string

const (
	CategoryStations Category = "stations"
	CategoryStorage  Category = "storage"
	CategoryCloud    Category = "cloud"
	CategoryOperator Category = "operator"
)

type Item struct {
	Category Category `json:"category"`
	ItemKey  string   `json:"item_key"`
	Status   Status   `json:"status"`
	Label    string   `json:"label"`
	Action   string   `json:"action"`
}

type Checklist struct {
	Status                Status    `json:"status"`
	Label                 string    `json:"label"`
	Action                string    `json:"action"`
	CheckedAt             time.Time `json:"checked_at"`
	SessionStartAvailable bool      `json:"session_start_available"`
	SessionStartAction    string    `json:"session_start_action"`
	Items                 []Item    `json:"items"`
}

type Builder struct {
	stationStore *stations.Store
	validator    stations.ReadinessValidator
	outputRoot   string
}

func NewBuilder(stationStore *stations.Store, validator stations.ReadinessValidator, outputRoot string) Builder {
	return Builder{stationStore: stationStore, validator: validator, outputRoot: outputRoot}
}

func (b Builder) Build() (Checklist, error) {
	if b.stationStore == nil {
		return Checklist{}, ErrUnavailable
	}
	stationList := b.stationStore.List()
	items := make([]Item, 0, len(stationList)*5+4)
	if len(stationList) != 3 {
		items = append(items, Item{Category: CategoryStations, ItemKey: "station_count", Status: StatusFailed, Label: "Jumlah station tidak tepat tiga", Action: "Restart aplikasi lalu periksa konfigurasi station."})
	}
	items = append(items, checkStorageRoot(b.outputRoot))
	for _, station := range stationList {
		stationReadiness := b.validator.Check(station)
		items = append(items, stationItem(station, stationReadiness))
		sawOutput := false
		for _, check := range stationReadiness.Checks {
			category := CategoryStations
			if check.CheckKey == "output_folder" {
				category = CategoryStorage
				sawOutput = true
			}
			items = append(items, Item{Category: category, ItemKey: station.StationID + "." + check.CheckKey, Status: fromStationStatus(check.Status), Label: station.Name + ": " + check.Label, Action: check.Action})
		}
		if !sawOutput {
			items = append(items, Item{Category: CategoryStorage, ItemKey: station.StationID + ".output_folder", Status: StatusFailed, Label: station.Name + ": Output folder tidak ditemukan dalam readiness station", Action: "Jalankan ulang station readiness atau restart aplikasi."})
		}
	}
	items = append(items,
		Item{Category: CategoryCloud, ItemKey: "supabase", Status: StatusPlaceholder, Label: "Supabase belum dicek otomatis", Action: "Metadata cloud perlu dikonfirmasi sebelum event."},
		Item{Category: CategoryCloud, ItemKey: "google_drive", Status: StatusPlaceholder, Label: "Google Drive belum dicek otomatis", Action: "Jalankan Drive connection check dan pastikan Google Drive authorized sebelum event."},
		Item{Category: CategoryOperator, ItemKey: "session_start", Status: StatusReady, Label: "Session start tersedia", Action: "Mulai session dari dashboard setelah station readiness aman dan operator mengonfirmasi device."},
	)

	status := aggregateStatus(items)
	label := "Event siap"
	action := "Tidak ada aksi diperlukan."
	switch status {
	case StatusFailed:
		label = "Event belum siap"
		action = "Perbaiki item gagal lalu jalankan checklist ulang."
	case StatusPlaceholder:
		label = "Event masih memiliki placeholder"
		action = "Konfirmasi item placeholder sebelum event."
	case StatusWarning, StatusUnknown:
		label = "Event belum sepenuhnya terverifikasi otomatis"
		action = "Konfirmasi item warning/unknown sebelum event."
	}

	return Checklist{Status: status, Label: label, Action: action, CheckedAt: time.Now().UTC(), SessionStartAvailable: true, SessionStartAction: "Session start tersedia setelah required readiness aman; konfirmasi warning/placeholder secara manual sebelum event.", Items: items}, nil
}

func checkStorageRoot(outputRoot string) Item {
	if outputRoot == "" {
		return Item{Category: CategoryStorage, ItemKey: "local_output_root", Status: StatusFailed, Label: "Local output root belum dikonfigurasi", Action: "Set local data output root lalu restart aplikasi."}
	}
	info, err := os.Stat(outputRoot)
	if err != nil || !info.IsDir() {
		return Item{Category: CategoryStorage, ItemKey: "local_output_root", Status: StatusFailed, Label: "Local output root tidak ditemukan atau bukan folder", Action: "Buat folder local output root lalu jalankan checklist ulang."}
	}
	probe, err := os.CreateTemp(outputRoot, ".selfstudio-root-check-*")
	if err != nil {
		return Item{Category: CategoryStorage, ItemKey: "local_output_root", Status: StatusFailed, Label: "Local output root tidak bisa ditulis", Action: "Perbaiki permission output root lalu jalankan checklist ulang."}
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return Item{Category: CategoryStorage, ItemKey: "local_output_root", Status: StatusFailed, Label: "Local output root gagal flush write probe", Action: "Perbaiki storage lalu jalankan checklist ulang."}
	}
	if err := os.Remove(name); err != nil {
		return Item{Category: CategoryStorage, ItemKey: "local_output_root", Status: StatusFailed, Label: "Local output root menyisakan file probe", Action: "Bersihkan probe file lalu jalankan checklist ulang."}
	}
	return Item{Category: CategoryStorage, ItemKey: "local_output_root", Status: StatusReady, Label: "Local output root bisa ditulis", Action: "Tidak ada aksi diperlukan."}
}

func aggregateStatus(items []Item) Status {
	status := StatusReady
	for _, item := range items {
		switch item.Status {
		case StatusFailed:
			return StatusFailed
		case StatusPlaceholder:
			if status != StatusFailed {
				status = StatusPlaceholder
			}
		case StatusWarning, StatusUnknown:
			if status == StatusReady {
				status = item.Status
			}
		}
	}
	return status
}

func stationItem(station stations.Station, readiness stations.Readiness) Item {
	status := fromStationStatus(readiness.Status)
	label := station.Name + ": " + readiness.Label
	action := readiness.Action
	for _, check := range readiness.Checks {
		if check.Status == stations.ReadinessFailed {
			label = station.Name + ": " + check.Label
			action = check.Action
			break
		}
	}
	return Item{Category: CategoryStations, ItemKey: station.StationID, Status: status, Label: label, Action: action}
}

func fromStationStatus(status stations.ReadinessStatus) Status {
	switch status {
	case stations.ReadinessReady:
		return StatusReady
	case stations.ReadinessWarning:
		return StatusWarning
	case stations.ReadinessFailed:
		return StatusFailed
	case stations.ReadinessUnknown:
		return StatusUnknown
	default:
		return StatusUnknown
	}
}

func DefaultOutputRoot(localDataDir string) string {
	return filepath.Join(localDataDir, "output")
}
