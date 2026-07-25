package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testDefaultBackupDir(t *testing.T, dir string) string {
	t.Helper()

	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "localapp"))

	backupDir, err := defaultBackupDirPath()
	if err != nil {
		t.Fatalf("defaultBackupDirPath returned error: %v", err)
	}
	return backupDir
}

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

func TestBuildCalendarHTMLShowsMonthEntries(t *testing.T) {
	html := buildCalendarHTML([]Entry{
		{ID: 1, Date: "2026-06-04", Text: "meeting / walk"},
		{ID: 2, Date: "2026-07-01", Text: "outside month"},
		{ID: 3, Date: "2026-06-05", Text: "<tag>"},
	}, 2026, 6)

	if !strings.Contains(html, "2026年06月") {
		t.Fatalf("calendar title missing month: %q", html)
	}
	if !strings.Contains(html, "meeting<br>walk") {
		t.Fatalf("calendar missing entry text: %q", html)
	}
	if !strings.Contains(html, `class="weekday sunday"`) || !strings.Contains(html, `class="weekday saturday"`) {
		t.Fatalf("calendar missing weekend weekday classes: %q", html)
	}
	if !strings.Contains(html, `class="day sunday"`) || !strings.Contains(html, `class="day saturday"`) {
		t.Fatalf("calendar missing weekend day classes: %q", html)
	}
	if strings.Contains(html, "outside month") {
		t.Fatalf("calendar included entry outside target month: %q", html)
	}
	if !strings.Contains(html, "&lt;tag&gt;") {
		t.Fatalf("calendar did not escape entry text: %q", html)
	}
}

func TestFormatCalendarEntryTextTreatsSlashAsLineBreak(t *testing.T) {
	got := formatCalendarEntryText("a / b / <tag>")
	want := "a<br>b<br>&lt;tag&gt;"
	if got != want {
		t.Fatalf("unexpected formatted entry: got %q want %q", got, want)
	}
}

func TestFormatEntryLineHighlightsToday(t *testing.T) {
	entry := Entry{ID: 3, Date: "2026-06-04", Text: "today note"}
	got := formatEntryLine(entry, Options{}, "2026-06-04", true)
	want := todayHighlightStart + "2026-06-04  today note" + todayHighlightEnd
	if got != want {
		t.Fatalf("unexpected highlighted line: got %q want %q", got, want)
	}
}

func TestFormatEntryLineDoesNotHighlightWhenDisabled(t *testing.T) {
	entry := Entry{ID: 3, Date: "2026-06-04", Text: "today note"}
	got := formatEntryLine(entry, Options{Numbered: true}, "2026-06-04", false)
	want := "3  2026-06-04  today note"
	if got != want {
		t.Fatalf("unexpected plain line: got %q want %q", got, want)
	}
}

type fakeCalendarSyncClient struct {
	registered map[string]bool
	inserted   []CalendarSyncItem
}

func (c *fakeCalendarSyncClient) ListDiaryItemKeys(ctx context.Context, start, end time.Time) (map[string]bool, error) {
	out := make(map[string]bool)
	for key, registered := range c.registered {
		out[key] = registered
	}
	return out, nil
}

func (c *fakeCalendarSyncClient) InsertDiaryItem(ctx context.Context, item CalendarSyncItem) error {
	c.inserted = append(c.inserted, item)
	return nil
}

func TestSyncEntriesToCalendarCreatesOnlyMissingMonthItems(t *testing.T) {
	registeredKey := diarySyncItemKey("2026-06-02", 1, "existing")
	client := &fakeCalendarSyncClient{
		registered: map[string]bool{registeredKey: true},
	}
	entries := []Entry{
		{ID: 1, Date: "2026-06-01", Text: "new / walk"},
		{ID: 2, Date: "2026-06-02", Text: "existing"},
		{ID: 3, Date: "2026-07-01", Text: "outside"},
		{ID: 4, Date: "2026-06-03", Text: "   "},
	}

	result, err := syncEntriesToCalendar(context.Background(), client, entries, 2026, time.June)
	if err != nil {
		t.Fatalf("syncEntriesToCalendar returned error: %v", err)
	}
	if result.Total != 3 || result.Existing != 1 || result.Created != 2 {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if len(client.inserted) != 2 {
		t.Fatalf("unexpected inserted item count: %+v", client.inserted)
	}
	if client.inserted[0].Date != "2026-06-01" || client.inserted[0].Part != 1 || client.inserted[0].Text != "new" {
		t.Fatalf("unexpected first inserted item: %+v", client.inserted[0])
	}
	if client.inserted[1].Date != "2026-06-01" || client.inserted[1].Part != 2 || client.inserted[1].Text != "walk" {
		t.Fatalf("unexpected second inserted item: %+v", client.inserted[1])
	}
}

func TestDiarySyncItemsForMonthSplitsSlashSeparatedItems(t *testing.T) {
	items := diarySyncItemsForMonth([]Entry{
		{ID: 1, Date: "2026-06-01", Text: "a / b / c"},
		{ID: 2, Date: "2026-07-01", Text: "outside"},
	}, 2026, time.June)

	if len(items) != 3 {
		t.Fatalf("unexpected item count: %+v", items)
	}
	for i, want := range []string{"a", "b", "c"} {
		if items[i].Text != want || items[i].Part != i+1 || items[i].Date != "2026-06-01" || items[i].Key == "" {
			t.Fatalf("unexpected item %d: %+v", i, items[i])
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

func TestPlatformConfigDirMacOS(t *testing.T) {
	got := platformConfigDir("darwin", "/Users/me", "")
	want := filepath.Join("/Users/me", "Library", "Application Support", "diary")
	if got != want {
		t.Fatalf("unexpected macOS config dir: got %q want %q", got, want)
	}
}

func TestPlatformConfigDirLinux(t *testing.T) {
	got := platformConfigDir("linux", "/home/me", "")
	want := filepath.Join("/home/me", ".config", "diary")
	if got != want {
		t.Fatalf("unexpected linux config dir: got %q want %q", got, want)
	}
}

func TestPlatformConfigDirWindows(t *testing.T) {
	got := platformConfigDir("windows", `C:\Users\me`, `C:\Users\me\AppData\Local`)
	want := filepath.Join(`C:\Users\me\AppData\Local`, "diary")
	if got != want {
		t.Fatalf("unexpected windows config dir: got %q want %q", got, want)
	}
}

func TestPrintHelpShowsCurrentConfigPathAndContents(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	cfgContent := "data_file = \"test.jsonl\"\nmax_len = 300\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	var out bytes.Buffer
	printHelpTo(&out, cfgPath, nil)
	got := out.String()

	if !strings.Contains(got, "設定ファイル:\n  "+cfgPath) {
		t.Fatalf("config path missing from help: %q", got)
	}
	if !strings.Contains(got, "現在の設定ファイルの内容:\n"+cfgContent) {
		t.Fatalf("config contents missing from help: %q", got)
	}
	if strings.Contains(got, "設定例:") || strings.Contains(got, "macOS:") {
		t.Fatalf("obsolete config example remains in help: %q", got)
	}
}

func TestPrintHelpReportsMissingConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")

	var out bytes.Buffer
	printHelpTo(&out, cfgPath, nil)
	got := out.String()

	if !strings.Contains(got, cfgPath) {
		t.Fatalf("config path missing from help: %q", got)
	}
	if !strings.Contains(got, "設定ファイルはまだ作成されていません") {
		t.Fatalf("missing config message not found in help: %q", got)
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

func TestParseArgsCalendar(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-v"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Calendar {
		t.Fatalf("expected calendar flag to be set: %+v", opts)
	}
}

func TestParseArgsCalendarWithMonth(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-v", "2026-03"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Calendar || opts.CalendarMonth != "2026-03" {
		t.Fatalf("unexpected calendar opts: %+v", opts)
	}
}

func TestParseArgsCalendarRejectsInvalidMonth(t *testing.T) {
	_, _, err := parseArgs([]string{"-v", "2026-13"})
	if err == nil {
		t.Fatal("expected parseArgs to reject invalid calendar month")
	}
}

func TestParseArgsGoogleSync(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"--sync-google"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.GoogleSync || opts.GoogleSyncMonth != "" {
		t.Fatalf("unexpected google sync opts: %+v", opts)
	}
}

func TestParseArgsGoogleSyncWithMonth(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"--sync-google", "2026-06"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.GoogleSync || opts.GoogleSyncMonth != "2026-06" {
		t.Fatalf("unexpected google sync opts: %+v", opts)
	}
}

func TestParseArgsGoogleSyncRejectsInvalidMonth(t *testing.T) {
	_, _, err := parseArgs([]string{"--sync-google", "2026-13"})
	if err == nil {
		t.Fatal("expected parseArgs to reject invalid google sync month")
	}
}

func TestParseArgsGoogleSyncRejectsMixedOptions(t *testing.T) {
	_, _, err := parseArgs([]string{"--sync-google", "-l"})
	if err == nil {
		t.Fatal("expected parseArgs to reject mixed google sync options")
	}
}

func TestParseArgsVersion(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"--version"})
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
	_, _, err := parseArgs([]string{"--version", "-l"})
	if err == nil {
		t.Fatal("expected parseArgs to reject mixed version options")
	}
}

func TestParseArgsCalendarRejectsMixedOptions(t *testing.T) {
	_, _, err := parseArgs([]string{"-v", "-l"})
	if err == nil {
		t.Fatal("expected parseArgs to reject mixed calendar options")
	}
}

func TestResolveCalendarMonth(t *testing.T) {
	now := time.Date(2026, 6, 4, 0, 0, 0, 0, time.Local)

	year, month, err := resolveCalendarMonth("", now)
	if err != nil {
		t.Fatalf("resolveCalendarMonth returned error: %v", err)
	}
	if year != 2026 || month != time.June {
		t.Fatalf("unexpected current month: %d %s", year, month)
	}

	year, month, err = resolveCalendarMonth("2026-03", now)
	if err != nil {
		t.Fatalf("resolveCalendarMonth returned error: %v", err)
	}
	if year != 2026 || month != time.March {
		t.Fatalf("unexpected specified month: %d %s", year, month)
	}
}

func TestLinuxCalendarHTMLDirUsesDownloadsWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	downloads := filepath.Join(dir, "Downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatalf("MkdirAll Downloads error: %v", err)
	}

	got, err := linuxCalendarHTMLDir()
	if err != nil {
		t.Fatalf("linuxCalendarHTMLDir returned error: %v", err)
	}

	want := filepath.Join(downloads, "diary")
	if got != want {
		t.Fatalf("unexpected linux calendar HTML dir: got %q want %q", got, want)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("calendar HTML dir was not created: info=%v err=%v", info, err)
	}
}

func TestLinuxCalendarHTMLDirFallsBackToHomeNonHiddenDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	got, err := linuxCalendarHTMLDir()
	if err != nil {
		t.Fatalf("linuxCalendarHTMLDir returned error: %v", err)
	}

	want := filepath.Join(dir, "diary-calendar")
	if got != want {
		t.Fatalf("unexpected linux calendar HTML fallback dir: got %q want %q", got, want)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("calendar HTML fallback dir was not created: info=%v err=%v", info, err)
	}
}

func TestParseArgsDeletePart(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-d", "101", "2"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Delete || opts.DeleteID != 101 || opts.DeletePart != 2 {
		t.Fatalf("unexpected delete opts: %+v", opts)
	}
}

func TestParseArgsDeleteToday(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-d", "today"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Delete || opts.DeleteDate != todayString() || opts.DeleteID != 0 {
		t.Fatalf("unexpected delete opts: %+v", opts)
	}
}

func TestParseArgsDeleteYesterdayPart(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-d", "yesterday", "2"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Delete || opts.DeleteDate != yesterdayString() || opts.DeletePart != 2 {
		t.Fatalf("unexpected delete opts: %+v", opts)
	}
}

func TestParseArgsDeletePartRejectsInvalidPart(t *testing.T) {
	_, _, err := parseArgs([]string{"-d", "101", "x"})
	if err == nil {
		t.Fatal("expected parseArgs to reject invalid delete part")
	}
}

func TestParseArgsAddForYesterday(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-a", "yesterday", "Work"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Add {
		t.Fatalf("expected add flag to be set: %+v", opts)
	}
	if opts.AddDate != yesterdayString() {
		t.Fatalf("unexpected add date: got %q want %q", opts.AddDate, yesterdayString())
	}
	if opts.AddText != "Work" {
		t.Fatalf("unexpected add text: %q", opts.AddText)
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

func TestParseArgsAppendForYesterday(t *testing.T) {
	opts, showHelp, err := parseArgs([]string{"-A", "yesterday", "Play"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if !opts.Append {
		t.Fatalf("expected append flag to be set: %+v", opts)
	}
	if opts.AddDate != yesterdayString() {
		t.Fatalf("unexpected append date: got %q want %q", opts.AddDate, yesterdayString())
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

func TestDeletePartByIDRemovesSlashSeparatedPart(t *testing.T) {
	entries := []Entry{
		{
			ID:        101,
			Date:      "2026-04-19",
			Text:      "a / b / c",
			CreatedAt: "2026-04-19T09:00:00+09:00",
			UpdatedAt: "2026-04-19T09:00:00+09:00",
		},
	}

	got, found, err := deletePartByID(entries, 101, 2)
	if err != nil {
		t.Fatalf("deletePartByID returned error: %v", err)
	}
	if !found {
		t.Fatal("expected entry to be found")
	}
	if got[0].Text != "a / c" {
		t.Fatalf("unexpected text: %q", got[0].Text)
	}
	if got[0].UpdatedAt == "2026-04-19T09:00:00+09:00" {
		t.Fatal("updated_at should be refreshed")
	}
}

func TestDeletePartByIDRejectsOutOfRangePart(t *testing.T) {
	entries := []Entry{{ID: 101, Date: "2026-04-19", Text: "a / b / c"}}

	got, found, err := deletePartByID(entries, 101, 4)
	if err == nil {
		t.Fatal("expected deletePartByID to reject out-of-range part")
	}
	if !found {
		t.Fatal("expected entry to be found")
	}
	if got[0].Text != "a / b / c" {
		t.Fatalf("text should be unchanged: %q", got[0].Text)
	}
}

func TestDeleteByDateRemovesEntry(t *testing.T) {
	entries := []Entry{
		{ID: 1, Date: "2026-04-18", Text: "a"},
		{ID: 2, Date: "2026-04-19", Text: "b"},
	}

	got, found := deleteByDate(entries, "2026-04-18")
	if !found {
		t.Fatal("expected entry to be found")
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("unexpected entries after delete: %+v", got)
	}
}

func TestDeletePartByDateRemovesSlashSeparatedPart(t *testing.T) {
	entries := []Entry{
		{
			ID:        101,
			Date:      "2026-04-19",
			Text:      "a / b / c",
			CreatedAt: "2026-04-19T09:00:00+09:00",
			UpdatedAt: "2026-04-19T09:00:00+09:00",
		},
	}

	got, found, err := deletePartByDate(entries, "2026-04-19", 2)
	if err != nil {
		t.Fatalf("deletePartByDate returned error: %v", err)
	}
	if !found {
		t.Fatal("expected entry to be found")
	}
	if got[0].Text != "a / c" {
		t.Fatalf("unexpected text: %q", got[0].Text)
	}
	if got[0].UpdatedAt == "2026-04-19T09:00:00+09:00" {
		t.Fatal("updated_at should be refreshed")
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
	dataFile := filepath.Join(dir, "diary.jsonl")
	backupDir := testDefaultBackupDir(t, dir)

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
	dataFile := filepath.Join(dir, "diary.jsonl")
	backupDir := testDefaultBackupDir(t, dir)

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
	dataFile := filepath.Join(dir, "diary.jsonl")
	backupDir := testDefaultBackupDir(t, dir)

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
	dataFile := filepath.Join(dir, "diary.jsonl")
	backupDir := testDefaultBackupDir(t, dir)

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
	dataFile := filepath.Join(dir, "diary.jsonl")
	backupDir := testDefaultBackupDir(t, dir)

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
