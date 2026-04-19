package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestParseArgsRestoreWithoutArgument(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-R"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Restore || opts.RestorePath != "" {
		t.Fatalf("unexpected restore opts: %+v", opts)
	}
}

func TestParseArgsVersion(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-v"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Version {
		t.Fatalf("expected version flag to be set: %+v", opts)
	}
}

func TestParseArgsVersionRejectsMixedOptions(t *testing.T) {
	_, _, err := parseArgs([]string{"-v", "-l"})
	if err == nil {
		t.Fatal("expected parseArgs to reject mixed version options")
	}
}

func TestParseArgsAppendForToday(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-A", "Play"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Append {
		t.Fatalf("expected append flag to be set: %+v", opts)
	}
	if opts.AddDate != todayString() {
		t.Fatalf("unexpected append date: got %q want %q", opts.AddDate, todayString())
	}
	if opts.AddText != "Play" {
		t.Fatalf("unexpected append text: %q", opts.AddText)
	}
}

func TestParseArgsAppendRejectsMixedOptions(t *testing.T) {
	_, _, err := parseArgs([]string{"-l", "-A", "Play"})
	if err == nil {
		t.Fatal("expected parseArgs to reject mixed append options")
	}
}

func TestAddOrUpdateEntryAppendExistingEntry(t *testing.T) {
	entries := []Entry{
		{
			ID:        1,
			Date:      "2026-04-19",
			Text:      "work",
			CreatedAt: "2026-04-19T09:00:00+09:00",
			UpdatedAt: "2026-04-19T09:00:00+09:00",
		},
	}

	err := addOrUpdateEntry(&entries, Options{
		Append:  true,
		AddDate: "2026-04-19",
		AddText: "Play",
	}, Config{MaxLen: 200})
	if err != nil {
		t.Fatalf("addOrUpdateEntry returned error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("unexpected entry count: got %d want 1", len(entries))
	}
	if entries[0].Text != "work / Play" {
		t.Fatalf("unexpected appended text: %q", entries[0].Text)
	}
	if entries[0].CreatedAt != "2026-04-19T09:00:00+09:00" {
		t.Fatalf("created_at should be preserved: %q", entries[0].CreatedAt)
	}
	if entries[0].UpdatedAt == "2026-04-19T09:00:00+09:00" {
		t.Fatal("updated_at should be refreshed")
	}
}

func TestAddOrUpdateEntryAppendCreatesEntryWhenMissing(t *testing.T) {
	var entries []Entry

	err := addOrUpdateEntry(&entries, Options{
		Append:  true,
		AddDate: "2026-04-19",
		AddText: "Play",
	}, Config{MaxLen: 200})
	if err != nil {
		t.Fatalf("addOrUpdateEntry returned error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("unexpected entry count: got %d want 1", len(entries))
	}
	if entries[0].Text != "Play" {
		t.Fatalf("unexpected created text: %q", entries[0].Text)
	}
	if entries[0].Date != "2026-04-19" {
		t.Fatalf("unexpected created date: %q", entries[0].Date)
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

func TestBackupEntriesPrunesOldBackupsToLimit(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "diary.jsonl")
	backupDir := filepath.Join(dir, "backups")
	entries := []Entry{{ID: 1, Date: "2026-04-15", Text: "note"}}

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("diary-backup-20200101-000000-%09d.jsonl", i)
		path := filepath.Join(backupDir, name)
		if err := writeBackupFile(path, entries); err != nil {
			t.Fatalf("writeBackupFile error: %v", err)
		}
	}

	newPath, err := backupEntries(entries, dataFile, backupDir)
	if err != nil {
		t.Fatalf("backupEntries returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(backupDir, "diary-backup-*.jsonl"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) != maxBackupHistory {
		t.Fatalf("unexpected backup count: got %d want %d", len(matches), maxBackupHistory)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new backup missing: %v", err)
	}
	oldest := filepath.Join(backupDir, "diary-backup-20200101-000000-000000000.jsonl")
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("expected oldest backup to be pruned, stat err=%v", err)
	}
}

func TestListBackupInfosReturnsNewestFirstWithCounts(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "localapp", "diary", "backups")
	dataFile := filepath.Join(dir, "diary.jsonl")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "localapp"))

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	olderPath := filepath.Join(backupDir, "diary-backup-20260415-101500-000000000.jsonl")
	newerPath := filepath.Join(backupDir, "diary-backup-20260415-121500-000000000.jsonl")
	if err := writeBackupFile(olderPath, []Entry{{ID: 1, Date: "2026-04-01", Text: "a"}}); err != nil {
		t.Fatalf("writeBackupFile older error: %v", err)
	}
	if err := writeBackupFile(newerPath, []Entry{
		{ID: 2, Date: "2026-04-02", Text: "b"},
		{ID: 3, Date: "2026-04-03", Text: "c"},
	}); err != nil {
		t.Fatalf("writeBackupFile newer error: %v", err)
	}

	backups, err := listBackupInfos(dataFile)
	if err != nil {
		t.Fatalf("listBackupInfos returned error: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("unexpected backup count: got %d want 2", len(backups))
	}
	if backups[0].Path != newerPath || backups[0].Count != 2 || backups[0].Index != 1 {
		t.Fatalf("unexpected newest backup: %+v", backups[0])
	}
	if backups[1].Path != olderPath || backups[1].Count != 1 || backups[1].Index != 2 {
		t.Fatalf("unexpected older backup: %+v", backups[1])
	}
}

func TestResolveRestorePathUsesBackupIndex(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "localapp", "diary", "backups")
	dataFile := filepath.Join(dir, "diary.jsonl")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "localapp"))

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	olderPath := filepath.Join(backupDir, "diary-backup-20260415-101500-000000000.jsonl")
	newerPath := filepath.Join(backupDir, "diary-backup-20260415-121500-000000000.jsonl")
	if err := writeBackupFile(olderPath, []Entry{{ID: 1, Date: "2026-04-01", Text: "a"}}); err != nil {
		t.Fatalf("writeBackupFile older error: %v", err)
	}
	if err := writeBackupFile(newerPath, []Entry{{ID: 2, Date: "2026-04-02", Text: "b"}}); err != nil {
		t.Fatalf("writeBackupFile newer error: %v", err)
	}

	got, err := resolveRestorePath(dataFile, "2")
	if err != nil {
		t.Fatalf("resolveRestorePath returned error: %v", err)
	}
	if got != olderPath {
		t.Fatalf("unexpected restore path: got %q want %q", got, olderPath)
	}
}

func TestPrintBackupListShowsNumberTimestampAndCount(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "localapp", "diary", "backups")
	dataFile := filepath.Join(dir, "diary.jsonl")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "localapp"))

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	path := filepath.Join(backupDir, "diary-backup-20260415-121500-000000000.jsonl")
	if err := writeBackupFile(path, []Entry{
		{ID: 1, Date: "2026-04-01", Text: "a"},
		{ID: 2, Date: "2026-04-02", Text: "b"},
	}); err != nil {
		t.Fatalf("writeBackupFile error: %v", err)
	}

	backups, err := listBackupInfos(dataFile)
	if err != nil {
		t.Fatalf("listBackupInfos returned error: %v", err)
	}

	var buf bytes.Buffer
	printBackupList(backups, &buf)

	got := buf.String()
	if !strings.Contains(got, "1  2026-04-15 12:15:00  2件") {
		t.Fatalf("backup list missing expected line: %q", got)
	}
}

func TestPromptRestorePathSelectsNumberWithoutReturningToShell(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "localapp", "diary", "backups")
	dataFile := filepath.Join(dir, "diary.jsonl")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "localapp"))

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	olderPath := filepath.Join(backupDir, "diary-backup-20260415-101500-000000000.jsonl")
	newerPath := filepath.Join(backupDir, "diary-backup-20260415-121500-000000000.jsonl")
	if err := writeBackupFile(olderPath, []Entry{{ID: 1, Date: "2026-04-01", Text: "a"}}); err != nil {
		t.Fatalf("writeBackupFile older error: %v", err)
	}
	if err := writeBackupFile(newerPath, []Entry{{ID: 2, Date: "2026-04-02", Text: "b"}}); err != nil {
		t.Fatalf("writeBackupFile newer error: %v", err)
	}

	var out bytes.Buffer
	got, err := promptRestorePath(dataFile, strings.NewReader("2\n"), &out)
	if err != nil {
		t.Fatalf("promptRestorePath returned error: %v", err)
	}
	if got != olderPath {
		t.Fatalf("unexpected restore path: got %q want %q", got, olderPath)
	}
	if !strings.Contains(out.String(), "restore> ") {
		t.Fatalf("prompt output missing restore prompt: %q", out.String())
	}
}

func TestPromptRestorePathRetriesUntilValidNumber(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "localapp", "diary", "backups")
	dataFile := filepath.Join(dir, "diary.jsonl")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "localapp"))

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	path := filepath.Join(backupDir, "diary-backup-20260415-121500-000000000.jsonl")
	if err := writeBackupFile(path, []Entry{{ID: 1, Date: "2026-04-02", Text: "b"}}); err != nil {
		t.Fatalf("writeBackupFile error: %v", err)
	}

	var out bytes.Buffer
	got, err := promptRestorePath(dataFile, strings.NewReader("x\n1\n"), &out)
	if err != nil {
		t.Fatalf("promptRestorePath returned error: %v", err)
	}
	if got != path {
		t.Fatalf("unexpected restore path: got %q want %q", got, path)
	}
	if !strings.Contains(out.String(), "1 から 1 の番号を入力してください。") {
		t.Fatalf("prompt output missing retry guidance: %q", out.String())
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
