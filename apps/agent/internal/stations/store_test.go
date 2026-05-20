package stations

import (
	"errors"
	"strings"
	"testing"
)

func TestNewStoreHasExactlyThreeStations(t *testing.T) {
	store := NewStore()
	stations := store.List()

	if len(stations) != 3 {
		t.Fatalf("len(stations) = %d, want 3", len(stations))
	}
	wantIDs := []string{Station1ID, Station2ID, Station3ID}
	for i, want := range wantIDs {
		if stations[i].StationID != want {
			t.Fatalf("stations[%d].StationID = %q, want %q", i, stations[i].StationID, want)
		}
	}
}

func TestUpdateStationValidatesRequiredFields(t *testing.T) {
	store := NewStore()
	_, err := store.Update(Station1ID, UpdateStation{
		Name:             "Station 1",
		DeviceIdentifier: "Sony A6000 #1",
		InputFolder:      "",
		BackgroundName:   "White",
		DefaultLUTPath:   "D:/luts/default.cube",
		OutputRule:       "{customer_name}/{order_number}/{station_id}",
	})

	var fieldErr FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("err = %v, want FieldError", err)
	}
}

func TestUpdateStationValidatesOutputRule(t *testing.T) {
	store := NewStore()
	_, err := store.Update(Station1ID, UpdateStation{
		Name:             "Station 1",
		DeviceIdentifier: "Sony A6000 #1",
		InputFolder:      "D:/input/main",
		BackgroundName:   "White",
		DefaultLUTPath:   "D:/luts/default.cube",
		OutputRule:       "../unsafe",
	})

	var fieldErr FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("err = %v, want FieldError", err)
	}
}

func TestUpdateStationValidatesFieldLength(t *testing.T) {
	store := NewStore()
	_, err := store.Update(Station1ID, UpdateStation{
		Name:             strings.Repeat("x", maxFieldLength+1),
		DeviceIdentifier: "Sony A6000 #1",
		InputFolder:      "D:/input/main",
		BackgroundName:   "White",
		DefaultLUTPath:   "D:/luts/default.cube",
		OutputRule:       "{station_id}",
	})

	var fieldErr FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("err = %v, want FieldError", err)
	}
}

func TestUpdateStationBlocksSlashBackslashDuplicateInputFolder(t *testing.T) {
	store := NewStore()
	_, err := store.Update(Station2ID, UpdateStation{
		Name:             "Station 2",
		DeviceIdentifier: "Sony A6000 #2",
		InputFolder:      "D:/Selfstudio/Input/Main",
		BackgroundName:   "White",
		DefaultLUTPath:   "D:/luts/default.cube",
		OutputRule:       "{station_id}",
	})
	if err != nil {
		t.Fatalf("Update station 2 error = %v", err)
	}
	_, err = store.Update(Station1ID, UpdateStation{
		Name:             "Station 1",
		DeviceIdentifier: "Sony A6000 #1",
		InputFolder:      `D:\Selfstudio\Input\Main`,
		BackgroundName:   "White",
		DefaultLUTPath:   "D:/luts/default.cube",
		OutputRule:       "{station_id}",
	})
	if !errors.Is(err, ErrDuplicateInputFolder) {
		t.Fatalf("err = %v, want ErrDuplicateInputFolder", err)
	}
}

func TestUpdateStationBlocksDuplicateInputFolder(t *testing.T) {
	store := NewStore()
	station2 := store.List()[1]
	_, err := store.Update(Station1ID, UpdateStation{
		Name:             "Station 1",
		DeviceIdentifier: "Sony A6000 #1",
		InputFolder:      station2.InputFolder,
		BackgroundName:   "White",
		DefaultLUTPath:   "D:/luts/default.cube",
		OutputRule:       "{customer_name}/{order_number}/{station_id}",
	})

	if !errors.Is(err, ErrDuplicateInputFolder) {
		t.Fatalf("err = %v, want ErrDuplicateInputFolder", err)
	}
}

func TestUpdateStationBlocksDuplicateCameraAssignment(t *testing.T) {
	store := NewStore()
	assignment := &CameraAssignment{IdentityKey: "WSL|USB:001,004|Sony_Alpha_A6000", CameraName: "Sony Alpha-A6000", Port: "usb:001,004", Runtime: "wsl"}
	_, err := store.Update(Station1ID, UpdateStation{Name: "Station 1", DeviceIdentifier: "Sony A6000 #1", InputFolder: "D:/input/one", BackgroundName: "White", DefaultLUTPath: "D:/luts/default.cube", OutputRule: "{station_id}", CameraAssignment: assignment})
	if err != nil {
		t.Fatalf("Update station 1 error = %v", err)
	}
	_, err = store.Update(Station2ID, UpdateStation{Name: "Station 2", DeviceIdentifier: "Sony A6000 #2", InputFolder: "D:/input/two", BackgroundName: "White", DefaultLUTPath: "D:/luts/default.cube", OutputRule: "{station_id}", CameraAssignment: &CameraAssignment{IdentityKey: "wsl|usb:001,004|sony_alpha_a6000", CameraName: "Sony Alpha-A6000", Port: "usb:001,004", Runtime: "wsl"}})
	if !errors.Is(err, ErrDuplicateCameraAssignment) {
		t.Fatalf("err = %v, want ErrDuplicateCameraAssignment", err)
	}
}

func TestUpdateStationAllowsEmptyCameraAssignments(t *testing.T) {
	store := NewStore()
	_, err := store.Update(Station1ID, UpdateStation{Name: "Station 1", DeviceIdentifier: "Sony A6000 #1", InputFolder: "D:/input/one", BackgroundName: "White", DefaultLUTPath: "D:/luts/default.cube", OutputRule: "{station_id}"})
	if err != nil {
		t.Fatalf("Update station 1 error = %v", err)
	}
	_, err = store.Update(Station2ID, UpdateStation{Name: "Station 2", DeviceIdentifier: "Sony A6000 #2", InputFolder: "D:/input/two", BackgroundName: "White", DefaultLUTPath: "D:/luts/default.cube", OutputRule: "{station_id}"})
	if err != nil {
		t.Fatalf("Update station 2 error = %v", err)
	}
}

func TestUpdateCameraAssignmentPersistsAssignment(t *testing.T) {
	store := NewStore()
	station, err := store.UpdateCameraAssignment(Station1ID, UpdateCameraAssignment{IdentityKey: "wsl|usb:001,004|sony_alpha_a6000", CameraName: "Sony Alpha-A6000", Port: "usb:001,004", Runtime: "wsl", Connected: true})
	if err != nil {
		t.Fatalf("UpdateCameraAssignment error = %v", err)
	}
	if station.CameraAssignment == nil || station.CameraAssignment.IdentityKey != "wsl|usb:001,004|sony_alpha_a6000" || !station.CameraAssignment.Connected {
		t.Fatalf("unexpected assignment: %+v", station.CameraAssignment)
	}
}

func TestUpdateStationPersistsValues(t *testing.T) {
	store := NewStore()
	updated, err := store.Update(Station1ID, UpdateStation{
		Name:             "Main Camera",
		DeviceIdentifier: "Sony A6000 Main",
		InputFolder:      "D:/Selfstudio/input/main",
		BackgroundName:   "Blue",
		DefaultLUTPath:   "D:/Selfstudio/luts/blue.cube",
		OutputRule:       "{customer_name}/{order_number}/{station_id}",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Main Camera" {
		t.Fatalf("Name = %q", updated.Name)
	}
	if got := store.List()[0].InputFolder; got != "D:/Selfstudio/input/main" {
		t.Fatalf("InputFolder = %q", got)
	}
}
