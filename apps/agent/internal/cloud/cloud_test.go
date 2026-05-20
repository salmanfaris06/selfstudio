package cloud

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultPersistenceNoSecretPublic(t *testing.T) {
	p := NewPersistence(t.TempDir())
	s, err := p.LoadOrDefault()
	if err != nil {
		t.Fatal(err)
	}
	dto := s.Public()
	if dto.ConnectionStatus != StatusNotConfigured || dto.CredentialsConfigured {
		t.Fatalf("unexpected dto: %+v", dto)
	}
}
func TestSaveConfigPublicExcludesSecrets(t *testing.T) {
	p := NewPersistence(t.TempDir())
	s := DefaultSettings()
	s.BucketName = "my-bucket"
	s.ServiceAccountJSON = `{"private_key":"secret"}`
	if err := p.Save(s); err != nil {
		t.Fatal(err)
	}
	got, _ := p.LoadOrDefault()
	b := got.Public()
	if !b.CredentialsConfigured {
		t.Fatal("expected configured")
	}
	serialized := b.Provider + b.DriveRootFolderID + b.DriveRootFolderName + b.FolderNamingTemplate + b.LastErrorCode + b.LastErrorAction
	if strings.Contains(serialized, "secret") {
		t.Fatal("secret leaked")
	}
}
func TestTargetRootRejectsTraversal(t *testing.T) {
	bad := []string{"../x", "a/./b", "a//b", "a\\..\\b", "ok/\u0001"}
	for _, v := range bad {
		if _, err := SanitizeTargetRoot(v); err == nil {
			t.Fatalf("expected error for %q", v)
		}
	}
}
func TestBuildObjectKeySanitizes(t *testing.T) {
	p, err := BuildObjectKey(PreviewInput{TargetRootPrefix: "root", CustomerName: " Jóhn Doe ", OrderNumber: "SO 1/2", StationID: "station-1", SessionID: "sess-1", AssetKind: "graded", FileName: "IMG 001.JPG", TakenAt: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	want := "root/2026/05/19/jóhn-doe/so-12/station-1/sess-1/graded/img-001.jpg"
	if p.ObjectKey != want {
		t.Fatalf("got %s", p.ObjectKey)
	}
	if strings.HasPrefix(p.ObjectKey, "/") {
		t.Fatal("leading slash")
	}
}
func TestBuildObjectKeyRejectsUnsafeFile(t *testing.T) {
	_, err := BuildObjectKey(PreviewInput{FileName: "..\\secret.jpg"})
	if err == nil {
		t.Fatal("expected unsafe file error")
	}
}
func TestBuildObjectKeyTooLong(t *testing.T) {
	_, err := BuildObjectKey(PreviewInput{TargetRootPrefix: strings.Repeat("a", 1100), FileName: "x.jpg"})
	if err == nil {
		t.Fatal("expected long error")
	}
}
