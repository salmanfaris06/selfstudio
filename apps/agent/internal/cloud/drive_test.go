package cloud

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultSettingsUseGoogleDrivePublicMetadataOnly(t *testing.T) {
	s := DefaultSettings()
	s.ServiceAccountJSON = `{"private_key":"secret"}`
	s.CredentialFilePath = `D:\secrets\drive-service-account.json`
	s.DriveRootFolderID = "root-folder-id"
	s.DriveRootFolderName = "Event Delivery"
	p := s.Public()

	if p.Provider != ProviderGoogleDrive {
		t.Fatalf("provider = %q", p.Provider)
	}
	if p.DriveRootFolderID != "root-folder-id" || p.DriveRootFolderName != "Event Delivery" {
		t.Fatalf("missing drive metadata: %+v", p)
	}
	serialized := p.Provider + p.DriveRootFolderID + p.DriveRootFolderName + p.FolderNamingTemplate + p.LastErrorCode + p.LastErrorAction
	if strings.Contains(serialized, "secret") || strings.Contains(serialized, "service-account") || strings.Contains(serialized, "private_key") {
		t.Fatalf("secret metadata leaked: %+v", p)
	}
}

func TestBuildDriveFolderPreviewSanitizesAndRejectsUnsafeSegments(t *testing.T) {
	preview, err := BuildDriveFolderPreview(PreviewInput{CustomerName: " Jóhn / Doe ", OrderNumber: "SO 1/2", StationID: "station-1", SessionID: "sess-1", TakenAt: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	want := "2026/05/20/jóhn-doe/so-12/station-1/sess-1"
	if preview.FolderPath != want {
		t.Fatalf("folder path = %q, want %q", preview.FolderPath, want)
	}
	if preview.Template != FolderNamingTemplate {
		t.Fatalf("template = %q", preview.Template)
	}

	_, err = BuildDriveFolderPreview(PreviewInput{CustomerName: "ok", OrderNumber: "../secret", StationID: "station-1", SessionID: "sess-1"})
	if err == nil {
		t.Fatal("expected unsafe traversal input to be rejected")
	}
}

func TestValidateSettingsRequiresGoogleDriveFolderRules(t *testing.T) {
	s := DefaultSettings()
	s.DriveRootFolderID = "folder-id"
	if err := ValidateSettings(s); err != nil {
		t.Fatalf("valid drive settings rejected: %v", err)
	}

	s.Provider = "gcs"
	if err := ValidateSettings(s); err == nil {
		t.Fatal("expected unsupported provider error")
	}

	s = DefaultSettings()
	s.DriveRootFolderID = ""
	if err := ValidateSettings(s); err == nil {
		t.Fatal("expected missing drive root folder error")
	}
}
