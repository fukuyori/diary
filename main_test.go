package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectEntriesSearchIsCaseInsensitive(t *testing.T) {
	entries := []Entry{
		{ID: 1, Date: "2026-03-01", Text: "Went for a Walk"},
		{ID: 2, Date: "2026-03-02", Text: "quiet day"},
		{ID: 3, Date: "2026-04-01", Text: "WALK by the river"},
	}

	got := collectEntries(entries, Options{
		Search:      true,
		SearchQuery: "walk",
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if got[0].ID != 3 || got[1].ID != 1 {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestCollectEntriesMonthAndSearch(t *testing.T) {
	entries := []Entry{
		{ID: 1, Date: "2026-03-01", Text: "coffee"},
		{ID: 2, Date: "2026-03-10", Text: "Coffee beans"},
		{ID: 3, Date: "2026-04-01", Text: "coffee"},
	}

	got := collectEntries(entries, Options{
		Search:      true,
		SearchQuery: "COFFEE",
		ListMonth:   "2026-03",
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	for _, entry := range got {
		if entry.Date[:7] != "2026-03" {
			t.Fatalf("entry outside month filter: %+v", entry)
		}
	}
}

func TestResolveBackupPathUsesDirectoryWhenNoExtension(t *testing.T) {
	dataFile := filepath.Join("data", "diary.jsonl")
	got, err := resolveBackupPath(dataFile, filepath.Join("tmp", "backups"))
	if err != nil {
		t.Fatalf("resolveBackupPath returned error: %v", err)
	}

	if filepath.Dir(got) != filepath.Join("tmp", "backups") {
		t.Fatalf("unexpected directory: %s", got)
	}
	if filepath.Ext(got) != ".jsonl" {
		t.Fatalf("unexpected backup extension: %s", got)
	}
}

func TestPlatformBackupDirWindows(t *testing.T) {
	got := platformBackupDir("windows", `C:\Users\me`, `C:\Users\me\AppData\Local`)
	want := filepath.Join(`C:\Users\me\AppData\Local`, "diary", "backups")
	if got != want {
		t.Fatalf("unexpected windows backup dir: got %q want %q", got, want)
	}
}

func TestPlatformBackupDirLinux(t *testing.T) {
	got := platformBackupDir("linux", "/home/me", "")
	want := filepath.Join("/home/me", ".local", "share", "diary", "backups")
	if got != want {
		t.Fatalf("unexpected linux backup dir: got %q want %q", got, want)
	}
}

func TestParseArgsRestore(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-R", "backup.jsonl"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Restore || opts.RestorePath != "backup.jsonl" {
		t.Fatalf("unexpected restore opts: %+v", opts)
	}
}

func TestRestoreEntriesRestoresBackupAndCreatesSafetyBackup(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "diary.jsonl")
	restoreFile := filepath.Join(dir, "backup.jsonl")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "localapp"))

	current := []Entry{
		{ID: 1, Date: "2026-03-01", Text: "current"},
	}
	restore := []Entry{
		{ID: 2, Date: "2026-04-01", Text: "restored"},
		{ID: 3, Date: "2026-04-02", Text: "restored 2"},
	}

	if err := writeEntriesFile(dataFile, current); err != nil {
		t.Fatalf("writeEntriesFile current error: %v", err)
	}
	if err := writeEntriesFile(restoreFile, restore); err != nil {
		t.Fatalf("writeEntriesFile restore error: %v", err)
	}

	safetyBackup, restoredCount, err := restoreEntries(dataFile, current, restoreFile)
	if err != nil {
		t.Fatalf("restoreEntries returned error: %v", err)
	}
	if restoredCount != len(restore) {
		t.Fatalf("unexpected restoredCount: got %d want %d", restoredCount, len(restore))
	}
	if _, err := os.Stat(safetyBackup); err != nil {
		t.Fatalf("safety backup not created: %v", err)
	}

	got, err := loadEntries(dataFile)
	if err != nil {
		t.Fatalf("loadEntries returned error: %v", err)
	}
	if len(got) != len(restore) || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("unexpected restored entries: %+v", got)
	}
}

func TestConfirmRestoreAcceptsExactDiary(t *testing.T) {
	inFile, err := os.CreateTemp(t.TempDir(), "confirm-in")
	if err != nil {
		t.Fatalf("CreateTemp input error: %v", err)
	}
	defer inFile.Close()
	if _, err := inFile.WriteString("diary\n"); err != nil {
		t.Fatalf("WriteString input error: %v", err)
	}
	if _, err := inFile.Seek(0, 0); err != nil {
		t.Fatalf("Seek input error: %v", err)
	}

	outFile, err := os.CreateTemp(t.TempDir(), "confirm-out")
	if err != nil {
		t.Fatalf("CreateTemp output error: %v", err)
	}
	defer outFile.Close()

	if err := confirmRestore(inFile, outFile); err != nil {
		t.Fatalf("confirmRestore returned error: %v", err)
	}
}

func TestConfirmRestoreRejectsUnexpectedInput(t *testing.T) {
	inFile, err := os.CreateTemp(t.TempDir(), "confirm-in")
	if err != nil {
		t.Fatalf("CreateTemp input error: %v", err)
	}
	defer inFile.Close()
	if _, err := inFile.WriteString("nope\n"); err != nil {
		t.Fatalf("WriteString input error: %v", err)
	}
	if _, err := inFile.Seek(0, 0); err != nil {
		t.Fatalf("Seek input error: %v", err)
	}

	outFile, err := os.CreateTemp(t.TempDir(), "confirm-out")
	if err != nil {
		t.Fatalf("CreateTemp output error: %v", err)
	}
	defer outFile.Close()

	err = confirmRestore(inFile, outFile)
	if err == nil {
		t.Fatal("expected confirmRestore to reject unexpected input")
	}
}
