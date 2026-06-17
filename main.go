// Copyright (c) 2026 Noriaki Fukuyori
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const appVersion = "2.0.0"
const maxBackupHistory = 10
const backupTimestampLayout = "20060102-150405-000000000"
const todayHighlightStart = "\x1b[1;33m"
const todayHighlightEnd = "\x1b[0m"

type BackupInfo struct {
	Index     int
	Path      string
	Timestamp time.Time
	Count     int
}

type Entry struct {
	ID        int    `json:"id"`
	Date      string `json:"date"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Config struct {
	DataFile              string `toml:"data_file"`
	MaxLen                int    `toml:"max_len"`
	GoogleCalendarID      string `toml:"google_calendar_id"`
	GoogleCredentialsFile string `toml:"google_credentials_file"`
	GoogleTokenFile       string `toml:"google_token_file"`
}

type Options struct {
	List              bool
	ListN             int
	ListLimitSet      bool
	ListMonth         string
	Reverse           bool
	Numbered          bool
	Search            bool
	SearchQuery       string
	InteractiveSearch bool
	Backup            bool
	BackupPath        string
	Restore           bool
	RestorePath       string
	Version           bool
	Calendar          bool
	CalendarMonth     string
	GoogleSync        bool
	GoogleSyncMonth   string

	Add     bool
	Append  bool
	AddDate string
	AddText string

	Delete     bool
	DeleteID   int
	DeleteDate string
	DeletePart int
}

func main() {
	opts, showHelp, err := parseArgs(os.Args[1:])
	if err != nil {
		exitErr("%v", err)
	}
	if showHelp {
		printHelp()
		return
	}
	if opts.Version {
		printVersion()
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		exitErr("設定読み込みエラー: %v", err)
	}

	entries, err := loadEntries(cfg.DataFile)
	if err != nil {
		exitErr("データ読み込みエラー: %v", err)
	}

	switch {
	case opts.GoogleSync:
		if err := runGoogleSync(context.Background(), entries, cfg, opts.GoogleSyncMonth, time.Now(), os.Stdin, os.Stdout); err != nil {
			exitErr("Googleカレンダー同期エラー: %v", err)
		}

	case opts.Calendar:
		if err := runCalendarGUI(entries, opts.CalendarMonth, time.Now()); err != nil {
			exitErr("カレンダー表示エラー: %v", err)
		}

	case opts.Add || opts.Append:
		if err := addOrUpdateEntry(&entries, opts, cfg); err != nil {
			exitErr("%v", err)
		}
		if err := saveWithAutomaticBackup(cfg.DataFile, entries); err != nil {
			exitErr("保存エラー: %v", err)
		}
		fmt.Println("保存しました。")

	case opts.Delete:
		var found bool
		if opts.DeletePart > 0 {
			var err error
			if opts.DeleteDate != "" {
				entries, found, err = deletePartByDate(entries, opts.DeleteDate, opts.DeletePart)
			} else {
				entries, found, err = deletePartByID(entries, opts.DeleteID, opts.DeletePart)
			}
			if err != nil {
				exitErr("%v", err)
			}
			if !found {
				exitErr("%s のデータは見つかりませんでした", deleteTargetLabel(opts))
			}
			if err := saveWithAutomaticBackup(cfg.DataFile, entries); err != nil {
				exitErr("保存エラー: %v", err)
			}
			fmt.Printf("%s の %d 番目の項目を削除しました。\n", deleteTargetLabel(opts), opts.DeletePart)
			break
		}

		if opts.DeleteDate != "" {
			entries, found = deleteByDate(entries, opts.DeleteDate)
		} else {
			entries, found = deleteByID(entries, opts.DeleteID)
		}
		if !found {
			exitErr("%s のデータは見つかりませんでした", deleteTargetLabel(opts))
		}
		if err := saveWithAutomaticBackup(cfg.DataFile, entries); err != nil {
			exitErr("保存エラー: %v", err)
		}
		fmt.Printf("%s を削除しました。\n", deleteTargetLabel(opts))

	case opts.Backup:
		if _, err := backupEntries(entries, cfg.DataFile, opts.BackupPath); err != nil {
			exitErr("バックアップエラー: %v", err)
		}

	case opts.Restore:
		var restorePath string
		if strings.TrimSpace(opts.RestorePath) == "" {
			restorePath, err = promptRestorePath(cfg.DataFile, os.Stdin, os.Stdout)
			if err != nil {
				exitErr("復元エラー: %v", err)
			}
			if restorePath == "" {
				fmt.Println("復元を中止しました。")
				break
			}
		} else {
			restorePath, err = resolveRestorePath(cfg.DataFile, opts.RestorePath)
			if err != nil {
				exitErr("復元エラー: %v", err)
			}
		}
		if err := confirmRestore(os.Stdin, os.Stdout); err != nil {
			exitErr("復元エラー: %v", err)
		}
		_, _, err = restoreEntries(cfg.DataFile, entries, restorePath)
		if err != nil {
			exitErr("復元エラー: %v", err)
		}
		fmt.Println("復元しました。")

	case opts.InteractiveSearch:
		if err := runInteractiveSearch(entries, opts); err != nil {
			exitErr("検索エラー: %v", err)
		}

	case opts.List || opts.Search:
		runList(entries, opts)

	default:
		printHelp()
	}
}

func parseArgs(args []string) (Options, bool, error) {
	var opts Options
	opts.ListN = 7

	if len(args) == 0 {
		return opts, true, nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-h", "--help":
			return opts, true, nil

		case "--sync-google":
			opts.GoogleSync = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				if !isYearMonth(args[i+1]) {
					return opts, false, errors.New("--sync-google の年月は YYYY-MM 形式で指定してください")
				}
				opts.GoogleSyncMonth = args[i+1]
				i++
			}

		case "-v", "--version":
			if arg == "--version" {
				opts.Version = true
			} else {
				opts.Calendar = true
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					if !isYearMonth(args[i+1]) {
						return opts, false, errors.New("-v の年月は YYYY-MM 形式で指定してください")
					}
					opts.CalendarMonth = args[i+1]
					i++
				}
			}

		case "-r":
			opts.Reverse = true

		case "-n":
			opts.Numbered = true

		case "-l":
			opts.List = true
			if i+1 < len(args) && isPositiveInt(args[i+1]) {
				n, _ := strconv.Atoi(args[i+1])
				opts.ListN = n
				opts.ListLimitSet = true
				i++
			}

		case "-m":
			if i+1 >= len(args) {
				return opts, false, errors.New("-m には YYYY-MM 形式の年月が必要です")
			}
			if !isYearMonth(args[i+1]) {
				return opts, false, errors.New("-m には YYYY-MM 形式の年月を指定してください")
			}
			opts.ListMonth = args[i+1]
			i++

		case "-s":
			if i+1 >= len(args) {
				return opts, false, errors.New("-s には検索語が必要です")
			}
			opts.Search = true
			opts.SearchQuery = args[i+1]
			i++

		case "-i":
			opts.InteractiveSearch = true

		case "-b":
			opts.Backup = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				opts.BackupPath = args[i+1]
				i++
			}

		case "-R", "--restore":
			opts.Restore = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				opts.RestorePath = args[i+1]
				i++
			}

		case "-a":
			if opts.Append || opts.Delete || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore || opts.GoogleSync {
				return opts, false, errors.New("-a は -A、一覧・検索・削除・バックアップ・復元系のオプションと同時に使えません")
			}
			opts.Add = true

			rest := args[i+1:]
			if len(rest) == 0 {
				text, err := promptText()
				if err != nil {
					return opts, false, err
				}
				opts.AddDate = todayString()
				opts.AddText = text
				return opts, false, nil
			}
			if len(rest) == 1 && rest[0] == "yesterday" {
				text, err := promptText()
				if err != nil {
					return opts, false, err
				}
				opts.AddDate = yesterdayString()
				opts.AddText = text
				return opts, false, nil
			}

			if len(rest) >= 2 && isAddDateArg(rest[0]) {
				opts.AddDate = resolveAddDateArg(rest[0])
				opts.AddText = strings.Join(rest[1:], " ")
			} else {
				opts.AddDate = todayString()
				opts.AddText = strings.Join(rest, " ")
			}

			if strings.TrimSpace(opts.AddText) == "" {
				return opts, false, errors.New("追加する本文が空です")
			}
			return opts, false, nil

		case "-A":
			if opts.Add || opts.Delete || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore || opts.GoogleSync {
				return opts, false, errors.New("-A は -a、一覧・検索・削除・バックアップ・復元系のオプションと同時に使えません")
			}
			opts.Append = true

			rest := args[i+1:]
			if len(rest) == 0 {
				text, err := promptText()
				if err != nil {
					return opts, false, err
				}
				opts.AddDate = todayString()
				opts.AddText = text
				return opts, false, nil
			}
			if len(rest) == 1 && rest[0] == "yesterday" {
				text, err := promptText()
				if err != nil {
					return opts, false, err
				}
				opts.AddDate = yesterdayString()
				opts.AddText = text
				return opts, false, nil
			}

			if len(rest) >= 2 && isAddDateArg(rest[0]) {
				opts.AddDate = resolveAddDateArg(rest[0])
				opts.AddText = strings.Join(rest[1:], " ")
			} else {
				opts.AddDate = todayString()
				opts.AddText = strings.Join(rest, " ")
			}

			if strings.TrimSpace(opts.AddText) == "" {
				return opts, false, errors.New("追記する本文が空です")
			}
			return opts, false, nil

		case "-d":
			if opts.Add || opts.Append || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore || opts.GoogleSync {
				return opts, false, errors.New("-d は追加・追記・一覧・検索・バックアップ・復元系のオプションと同時に使えません")
			}
			opts.Delete = true

			if i+1 >= len(args) {
				return opts, false, errors.New("-d には削除対象のシリアル番号、today、yesterday のいずれかが必要です")
			}
			if isDeleteDateArg(args[i+1]) {
				opts.DeleteDate = resolveDeleteDateArg(args[i+1])
			} else {
				if !isPositiveInt(args[i+1]) {
					return opts, false, errors.New("-d には正の整数のシリアル番号、today、yesterday のいずれかを指定してください")
				}
				id, _ := strconv.Atoi(args[i+1])
				opts.DeleteID = id
			}
			if i+2 < len(args) {
				if i+3 < len(args) {
					return opts, false, errors.New("-d の引数が多すぎます")
				}
				if !isPositiveInt(args[i+2]) {
					return opts, false, errors.New("-d の第2引数には正の整数の項目番号を指定してください")
				}
				part, _ := strconv.Atoi(args[i+2])
				opts.DeletePart = part
			}
			return opts, false, nil

		default:
			return opts, false, fmt.Errorf("不明な引数です: %s", arg)
		}
	}

	if opts.Reverse && !opts.List {
		if !opts.Search && !opts.InteractiveSearch {
			return opts, false, errors.New("-r は -l、-s、-i のいずれかと一緒に使ってください")
		}
	}
	if opts.Numbered && !opts.List && !opts.Search && !opts.InteractiveSearch {
		return opts, false, errors.New("-n は -l、-s、-i のいずれかと一緒に使ってください")
	}
	if opts.ListMonth != "" && !opts.List && !opts.Search && !opts.InteractiveSearch {
		return opts, false, errors.New("-m は -l、-s、-i のいずれかと一緒に使ってください")
	}
	if opts.Search && strings.TrimSpace(opts.SearchQuery) == "" {
		return opts, false, errors.New("検索語が空です")
	}
	if opts.Search && opts.InteractiveSearch {
		return opts, false, errors.New("-s と -i は同時に使えません")
	}
	if opts.Backup && (opts.List || opts.Search || opts.InteractiveSearch || opts.ListMonth != "" || opts.Reverse || opts.Numbered) {
		return opts, false, errors.New("-b は一覧・検索系のオプションと同時に使えません")
	}
	if opts.Backup && (opts.Add || opts.Append || opts.Delete) {
		return opts, false, errors.New("-b は -a、-A、-d と同時に使えません")
	}
	if opts.Version && (opts.Add || opts.Append || opts.Delete || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore || opts.Calendar || opts.GoogleSync || opts.ListMonth != "" || opts.Reverse || opts.Numbered || opts.ListLimitSet) {
		return opts, false, errors.New("--version は単独で使ってください")
	}
	if opts.Calendar && (opts.Add || opts.Append || opts.Delete || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore || opts.Version || opts.GoogleSync || opts.ListMonth != "" || opts.Reverse || opts.Numbered || opts.ListLimitSet) {
		return opts, false, errors.New("-v は単独で使ってください")
	}
	if opts.GoogleSync && (opts.Add || opts.Append || opts.Delete || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore || opts.Version || opts.Calendar || opts.ListMonth != "" || opts.Reverse || opts.Numbered || opts.ListLimitSet) {
		return opts, false, errors.New("--sync-google は単独で使ってください")
	}
	if opts.Restore && (opts.List || opts.Search || opts.InteractiveSearch || opts.ListMonth != "" || opts.Reverse || opts.Numbered) {
		return opts, false, errors.New("-R は一覧・検索系のオプションと同時に使えません")
	}
	if opts.Restore && (opts.Add || opts.Append || opts.Delete || opts.Backup) {
		return opts, false, errors.New("-R は -a、-A、-d、-b と同時に使えません")
	}
	return opts, false, nil
}

func addOrUpdateEntry(entries *[]Entry, opts Options, cfg Config) error {
	date := opts.AddDate
	text := strings.TrimSpace(opts.AddText)

	if !isDate(date) {
		return fmt.Errorf("日付は YYYY-MM-DD 形式で指定してください")
	}
	if text == "" {
		return fmt.Errorf("本文が空です")
	}
	if utf8Len(text) > cfg.MaxLen {
		return fmt.Errorf("本文は %d 文字以内にしてください", cfg.MaxLen)
	}

	now := time.Now().Format(time.RFC3339)
	es := *entries

	for i := range es {
		if es[i].Date == date {
			if opts.Append && strings.TrimSpace(es[i].Text) != "" {
				text = es[i].Text + " / " + text
			}
			if utf8Len(text) > cfg.MaxLen {
				return fmt.Errorf("本文は %d 文字以内にしてください", cfg.MaxLen)
			}
			es[i].Text = text
			es[i].UpdatedAt = now
			*entries = es
			return nil
		}
	}

	newEntry := Entry{
		ID:        nextID(es),
		Date:      date,
		Text:      text,
		CreatedAt: now,
		UpdatedAt: now,
	}
	es = append(es, newEntry)
	*entries = es
	return nil
}

func deleteByID(entries []Entry, id int) ([]Entry, bool) {
	out := make([]Entry, 0, len(entries))
	deleted := false

	for _, e := range entries {
		if e.ID == id {
			deleted = true
			continue
		}
		out = append(out, e)
	}
	return out, deleted
}

func deleteByDate(entries []Entry, date string) ([]Entry, bool) {
	out := make([]Entry, 0, len(entries))
	deleted := false

	for _, e := range entries {
		if e.Date == date {
			deleted = true
			continue
		}
		out = append(out, e)
	}
	return out, deleted
}

func deletePartByID(entries []Entry, id, part int) ([]Entry, bool, error) {
	return deletePart(entries, part, func(entry Entry) bool {
		return entry.ID == id
	}, fmt.Sprintf("ID %d", id))
}

func deletePartByDate(entries []Entry, date string, part int) ([]Entry, bool, error) {
	return deletePart(entries, part, func(entry Entry) bool {
		return entry.Date == date
	}, date)
}

func deletePart(entries []Entry, part int, match func(Entry) bool, label string) ([]Entry, bool, error) {
	if part <= 0 {
		return entries, false, errors.New("削除する項目番号は正の整数で指定してください")
	}

	for i := range entries {
		if !match(entries[i]) {
			continue
		}

		parts := splitEntryText(entries[i].Text)
		if part > len(parts) {
			return entries, true, fmt.Errorf("%s には %d 番目の項目がありません", label, part)
		}

		parts = append(parts[:part-1], parts[part:]...)
		entries[i].Text = strings.Join(parts, " / ")
		entries[i].UpdatedAt = time.Now().Format(time.RFC3339)
		return entries, true, nil
	}

	return entries, false, nil
}

func deleteTargetLabel(opts Options) string {
	if opts.DeleteDate != "" {
		return opts.DeleteDate
	}
	return fmt.Sprintf("ID %d", opts.DeleteID)
}

func splitEntryText(text string) []string {
	rawParts := strings.Split(text, "/")
	parts := make([]string, 0, len(rawParts))
	for _, rawPart := range rawParts {
		parts = append(parts, strings.TrimSpace(rawPart))
	}
	return parts
}

func printHelp() {
	fmt.Printf(`1行日記 CLI v%s

使い方:
  diary
      ヘルプを表示

  diary -v [YYYY-MM]
      GUIで当月または指定年月のカレンダーを表示

  diary --version
      バージョンを表示

  diary --sync-google [YYYY-MM]
      当月または指定年月の日記をGoogleカレンダーに同期
      "/" 区切りの未登録項目だけ終日イベントとして作成

  diary -l [件数]
      直近の記録を古いもの順で表示
      件数省略時は 7

  diary -m YYYY-MM -l [件数]
      指定した年月の記録を表示
      件数省略時はその月を全件表示

  diary -s "検索語"
      本文を大文字小文字を区別せず検索

  diary -i
      対話的に絞り込み検索

  diary -b [保存先]
      その場でバックアップを作成
      保存先省略時はOSごとのローカル保存先に保存

  diary -R [バックアップファイル]
      引数省略時はバックアップ一覧を表示して番号入力で復元
      バックアップファイル指定でも復元可能
      復元前のデータは自動でバックアップ
      実行前に "diary" の入力確認あり

  diary -r -l [件数]
      直近の記録を新しいもの順で表示

  diary -n -l [件数]
      直近の記録を古いもの順・シリアル番号付きで表示

  diary -r -n -l [件数]
      直近の記録を新しいもの順・シリアル番号付きで表示

  diary -a "本文"
      今日の日付で追加または上書き

  diary -a yesterday "本文"
      前日の日付で追加または上書き

  diary -a YYYY-MM-DD "本文"
      指定日で追加または上書き

  diary -A "本文"
      今日の日付で既存本文の末尾に " / 本文" を追記
      未登録ならそのまま追加

  diary -A yesterday "本文"
      前日の日付で既存本文の末尾に " / 本文" を追記
      未登録ならそのまま追加

  diary -A YYYY-MM-DD "本文"
      指定日で既存本文の末尾に " / 本文" を追記
      未登録ならそのまま追加

  diary -d ID
      指定したシリアル番号の記録を削除

  diary -d today
      今日の記録を削除

  diary -d yesterday
      前日の記録を削除

  diary -d ID N
      指定したシリアル番号の記録から "/" 区切りの N 番目の項目を削除
      N は 1 から数えます

  diary -d today N
      今日の記録から "/" 区切りの N 番目の項目を削除

  diary -d yesterday N
      前日の記録から "/" 区切りの N 番目の項目を削除

設定ファイル:
  macOS: ~/Library/Application Support/diary/config.toml
  Linux: ~/.config/diary/config.toml
  Windows: %%LOCALAPPDATA%%\diary\config.toml

設定例:
  data_file = "C:\\Users\\yourname\\diary\\diary.jsonl"
  max_len = 200
  google_calendar_id = "primary"
`, appVersion)
}

func printVersion() {
	fmt.Printf("diary v%s\n", appVersion)
}

type CalendarSyncClient interface {
	ListDiaryItemKeys(ctx context.Context, start, end time.Time) (map[string]bool, error)
	InsertDiaryItem(ctx context.Context, item CalendarSyncItem) error
}

func runGoogleSync(ctx context.Context, entries []Entry, cfg Config, monthArg string, now time.Time, in io.Reader, out io.Writer) error {
	year, month, err := resolveCalendarMonth(monthArg, now)
	if err != nil {
		return err
	}

	client, err := newGoogleCalendarClient(ctx, cfg, in, out)
	if err != nil {
		return err
	}

	result, err := syncEntriesToCalendar(ctx, client, entries, year, month)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%04d-%02d の同期: 対象 %d件、登録済み %d件、新規登録 %d件\n", year, int(month), result.Total, result.Existing, result.Created)
	return nil
}

type CalendarSyncResult struct {
	Total    int
	Existing int
	Created  int
}

type CalendarSyncItem struct {
	EntryID int
	Date    string
	Part    int
	Text    string
	Key     string
}

func syncEntriesToCalendar(ctx context.Context, client CalendarSyncClient, entries []Entry, year int, month time.Month) (CalendarSyncResult, error) {
	monthItems := diarySyncItemsForMonth(entries, year, month)
	start, end := monthTimeRange(year, month)
	registered, err := client.ListDiaryItemKeys(ctx, start, end)
	if err != nil {
		return CalendarSyncResult{}, err
	}

	result := CalendarSyncResult{Total: len(monthItems)}
	for _, item := range monthItems {
		if registered[item.Key] {
			result.Existing++
			continue
		}
		if err := client.InsertDiaryItem(ctx, item); err != nil {
			return result, err
		}
		registered[item.Key] = true
		result.Created++
	}
	return result, nil
}

func entriesForMonth(entries []Entry, year int, month time.Month) []Entry {
	prefix := fmt.Sprintf("%04d-%02d-", year, int(month))
	out := make([]Entry, 0)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Date, prefix) && strings.TrimSpace(entry.Text) != "" {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date == out[j].Date {
			return out[i].ID < out[j].ID
		}
		return out[i].Date < out[j].Date
	})
	return out
}

func diarySyncItemsForMonth(entries []Entry, year int, month time.Month) []CalendarSyncItem {
	monthEntries := entriesForMonth(entries, year, month)
	items := make([]CalendarSyncItem, 0)
	for _, entry := range monthEntries {
		partNumber := 0
		for _, part := range splitEntryText(entry.Text) {
			if part == "" {
				continue
			}
			partNumber++
			items = append(items, CalendarSyncItem{
				EntryID: entry.ID,
				Date:    entry.Date,
				Part:    partNumber,
				Text:    part,
				Key:     diarySyncItemKey(entry.Date, partNumber, part),
			})
		}
	}
	return items
}

func diarySyncItemKey(date string, part int, text string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", date, part, strings.TrimSpace(text))))
	return hex.EncodeToString(sum[:])
}

func monthTimeRange(year int, month time.Month) (time.Time, time.Time) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	return start, start.AddDate(0, 1, 0)
}

type GoogleCalendarClient struct {
	service    *calendar.Service
	calendarID string
}

func newGoogleCalendarClient(ctx context.Context, cfg Config, in io.Reader, out io.Writer) (*GoogleCalendarClient, error) {
	credentialsPath := expandHomePath(cfg.GoogleCredentialsFile)
	tokenPath := expandHomePath(cfg.GoogleTokenFile)
	if strings.TrimSpace(credentialsPath) == "" {
		return nil, errors.New("google_credentials_file が設定されていません")
	}
	if strings.TrimSpace(tokenPath) == "" {
		return nil, errors.New("google_token_file が設定されていません")
	}

	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("Google OAuth認証情報を読み込めません: %w", err)
	}
	oauthConfig, err := google.ConfigFromJSON(credentials, calendar.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("Google OAuth認証情報を解析できません: %w", err)
	}

	httpClient, err := oauthHTTPClient(ctx, oauthConfig, tokenPath, in, out)
	if err != nil {
		return nil, err
	}
	service, err := calendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	calendarID := strings.TrimSpace(cfg.GoogleCalendarID)
	if calendarID == "" {
		calendarID = "primary"
	}
	return &GoogleCalendarClient{
		service:    service,
		calendarID: calendarID,
	}, nil
}

func (c *GoogleCalendarClient) ListDiaryItemKeys(ctx context.Context, start, end time.Time) (map[string]bool, error) {
	registered := make(map[string]bool)
	call := c.service.Events.List(c.calendarID).
		Context(ctx).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		PrivateExtendedProperty("diary_app=diary")

	pageToken := ""
	for {
		if pageToken != "" {
			call.PageToken(pageToken)
		}
		events, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, event := range events.Items {
			if event.ExtendedProperties == nil || event.ExtendedProperties.Private == nil {
				continue
			}
			key := event.ExtendedProperties.Private["diary_item_key"]
			if key != "" {
				registered[key] = true
			}
		}
		if events.NextPageToken == "" {
			return registered, nil
		}
		pageToken = events.NextPageToken
	}
}

func (c *GoogleCalendarClient) InsertDiaryItem(ctx context.Context, item CalendarSyncItem) error {
	start, err := time.Parse("2006-01-02", item.Date)
	if err != nil {
		return err
	}
	end := start.AddDate(0, 0, 1)
	event := &calendar.Event{
		Summary:     item.Text,
		Description: fmt.Sprintf("diary %s", item.Date),
		Start:       &calendar.EventDateTime{Date: item.Date},
		End:         &calendar.EventDateTime{Date: end.Format("2006-01-02")},
		ExtendedProperties: &calendar.EventExtendedProperties{
			Private: map[string]string{
				"diary_app":      "diary",
				"diary_id":       strconv.Itoa(item.EntryID),
				"diary_date":     item.Date,
				"diary_part":     strconv.Itoa(item.Part),
				"diary_item_key": item.Key,
			},
		},
	}
	_, err = c.service.Events.Insert(c.calendarID, event).Context(ctx).Do()
	return err
}

func oauthHTTPClient(ctx context.Context, oauthConfig *oauth2.Config, tokenPath string, in io.Reader, out io.Writer) (*http.Client, error) {
	token, err := tokenFromFile(tokenPath)
	if err != nil {
		token, err = tokenFromWeb(oauthConfig, in, out)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenPath, token); err != nil {
			return nil, err
		}
	}
	return oauthConfig.Client(ctx, token), nil
}

func tokenFromFile(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

func tokenFromWeb(oauthConfig *oauth2.Config, _ io.Reader, out io.Writer) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("ローカル認証サーバーを開始できません: %w", err)
	}

	state, err := randomOAuthState()
	if err != nil {
		listener.Close()
		return nil, err
	}

	localConfig := *oauthConfig
	localConfig.RedirectURL = "http://" + listener.Addr().String() + "/oauth2callback"

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			sendOAuthError(errCh, errors.New("Google認証のstateが一致しません"))
			return
		}
		if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
			http.Error(w, oauthErr, http.StatusBadRequest)
			sendOAuthError(errCh, fmt.Errorf("Google認証が拒否されました: %s", oauthErr))
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			sendOAuthError(errCh, errors.New("Google認証コードを受け取れませんでした"))
			return
		}

		fmt.Fprintln(w, "認証しました。このウィンドウを閉じてdiaryに戻ってください。")
		codeCh <- code
	})
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sendOAuthError(errCh, err)
		}
	}()
	defer shutdownOAuthServer(server)

	authURL := localConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Fprintf(out, "Google認証URLを開いて許可してください:\n%s\n", authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, errors.New("Google認証がタイムアウトしました")
	}

	token, err := localConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("Google認証コードをトークンに交換できません: %w", err)
	}
	return token, nil
}

func randomOAuthState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("Google認証stateを生成できません: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func sendOAuthError(errCh chan<- error, err error) {
	select {
	case errCh <- err:
	default:
	}
}

func shutdownOAuthServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func saveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(token)
}

func runCalendarGUI(entries []Entry, monthArg string, now time.Time) error {
	year, month, err := resolveCalendarMonth(monthArg, now)
	if err != nil {
		return err
	}
	htmlText := buildCalendarHTML(entries, year, month)

	dir, err := calendarHTMLDir()
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, fmt.Sprintf("diary-calendar-%04d-%02d-*.html", year, int(month)))
	if err != nil {
		return err
	}
	path := file.Name()
	if _, err := file.WriteString(htmlText); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := os.Chmod(path, 0o644); err != nil {
		_ = os.Remove(path)
		return err
	}

	if err := openInBrowser(path); err != nil {
		return err
	}
	fmt.Printf("カレンダーを開きました: %s\n", path)
	return nil
}

func calendarHTMLDir() (string, error) {
	if runtime.GOOS == "linux" {
		return linuxCalendarHTMLDir()
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "diary")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func linuxCalendarHTMLDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	downloads := filepath.Join(home, "Downloads")
	if info, err := os.Stat(downloads); err == nil && info.IsDir() {
		return ensureDir(filepath.Join(downloads, "diary"))
	}
	return ensureDir(filepath.Join(home, "diary-calendar"))
}

func ensureDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func resolveCalendarMonth(monthArg string, now time.Time) (int, time.Month, error) {
	if strings.TrimSpace(monthArg) == "" {
		year, month, _ := now.Date()
		return year, month, nil
	}
	parsed, err := time.Parse("2006-01", monthArg)
	if err != nil {
		return 0, 0, errors.New("年月は YYYY-MM 形式で指定してください")
	}
	year, month, _ := parsed.Date()
	return year, month, nil
}

func openInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func buildCalendarHTML(entries []Entry, year int, month time.Month) string {
	entriesByDate := make(map[string]Entry)
	for _, entry := range entries {
		entriesByDate[entry.Date] = entry
	}

	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	daysInMonth := first.AddDate(0, 1, -1).Day()
	leadingDays := int(first.Weekday())
	today := todayString()

	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>diary %04d-%02d</title>
<style>
:root { color-scheme: light; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
body { margin: 0; background: #f6f7f9; color: #20242a; }
main { max-width: 1180px; margin: 0 auto; padding: 28px; }
h1 { margin: 0 0 18px; font-size: 28px; font-weight: 700; }
.calendar { display: grid; grid-template-columns: repeat(7, minmax(120px, 1fr)); gap: 1px; background: #cfd5dc; border: 1px solid #cfd5dc; }
.weekday, .day { background: #fff; }
.weekday { padding: 10px; text-align: center; font-weight: 700; color: #59616b; }
.weekday.sunday, .day.sunday .date { color: #c3312f; }
.weekday.saturday, .day.saturday .date { color: #2468c9; }
.day { min-height: 126px; padding: 10px; }
.empty { background: #eef1f4; }
.date { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; margin-bottom: 8px; border-radius: 50%%; font-weight: 700; }
.today .date { background: #20242a; color: #fff; }
.today.sunday .date { background: #c3312f; color: #fff; }
.today.saturday .date { background: #2468c9; color: #fff; }
.entry { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.45; font-size: 14px; color: #222831; }
.missing { color: #a0a7b0; font-size: 13px; }
@media (max-width: 760px) {
  main { padding: 14px; }
  .calendar { grid-template-columns: 1fr; background: transparent; border: 0; gap: 8px; }
  .weekday, .empty { display: none; }
  .day { min-height: 0; border: 1px solid #d8dde3; }
}
</style>
</head>
<body>
<main>
<h1>%04d年%02d月</h1>
<section class="calendar" aria-label="%04d年%02d月のカレンダー">
`, year, int(month), year, int(month), year, int(month))

	for i, weekday := range []string{"日", "月", "火", "水", "木", "金", "土"} {
		classes := weekdayClasses(i)
		fmt.Fprintf(&b, `<div class="%s">%s</div>`+"\n", classes, weekday)
	}
	for i := 0; i < leadingDays; i++ {
		b.WriteString(`<div class="day empty" aria-hidden="true"></div>` + "\n")
	}
	for day := 1; day <= daysInMonth; day++ {
		date := fmt.Sprintf("%04d-%02d-%02d", year, int(month), day)
		classes := "day"
		switch time.Date(year, month, day, 0, 0, 0, 0, time.Local).Weekday() {
		case time.Sunday:
			classes += " sunday"
		case time.Saturday:
			classes += " saturday"
		}
		if date == today {
			classes += " today"
		}
		fmt.Fprintf(&b, `<article class="%s">`+"\n", classes)
		fmt.Fprintf(&b, `<div class="date">%d</div>`+"\n", day)
		if entry, ok := entriesByDate[date]; ok && strings.TrimSpace(entry.Text) != "" {
			fmt.Fprintf(&b, `<div class="entry">%s</div>`+"\n", formatCalendarEntryText(entry.Text))
		} else {
			b.WriteString(`<div class="missing">未登録</div>` + "\n")
		}
		b.WriteString("</article>\n")
	}

	b.WriteString(`</section>
</main>
</body>
</html>
`)
	return b.String()
}

func weekdayClasses(index int) string {
	switch index {
	case 0:
		return "weekday sunday"
	case 6:
		return "weekday saturday"
	default:
		return "weekday"
	}
}

func formatCalendarEntryText(text string) string {
	parts := splitEntryText(text)
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		lines = append(lines, html.EscapeString(part))
	}
	return strings.Join(lines, "<br>")
}

func loadEntries(path string) ([]Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Entry{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func saveEntries(path string, entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Date < entries[j].Date
	})

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			f.Close()
			_ = os.Remove(tmpPath)
			return err
		}
		if _, err := w.WriteString(string(b) + "\n"); err != nil {
			f.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}

	if err := w.Flush(); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}

	_ = os.Remove(tmpPath)
	return writeEntriesFile(path, entries)
}

func saveWithAutomaticBackup(dataFile string, entries []Entry) error {
	if err := saveEntries(dataFile, entries); err != nil {
		return err
	}

	if _, err := backupEntries(entries, dataFile, ""); err != nil {
		return fmt.Errorf("保存は完了しましたが自動バックアップに失敗しました: %w", err)
	}

	return nil
}

func confirmRestore(in *os.File, out *os.File) error {
	fmt.Fprintln(out, `復元を実行すると現在のデータがバックアップ元で置き換わります。続けるには "diary" と入力してください。`)
	fmt.Fprint(out, "confirm> ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("確認入力を読み取れませんでした: %w", err)
	}
	if strings.TrimSpace(line) != "diary" {
		return errors.New(`確認に失敗したため復元を中止しました`)
	}
	return nil
}

func printBackupList(backups []BackupInfo, out io.Writer) {
	if len(backups) == 0 {
		fmt.Fprintln(out, "バックアップはありません。")
		return
	}

	fmt.Fprintln(out, "バックアップ一覧:")
	for _, backup := range backups {
		fmt.Fprintf(out, "%d  %s  %d件\n", backup.Index, backup.Timestamp.Format("2006-01-02 15:04:05"), backup.Count)
	}
}

func promptRestorePath(dataFile string, in io.Reader, out io.Writer) (string, error) {
	backups, err := listBackupInfos(dataFile)
	if err != nil {
		return "", err
	}
	printBackupList(backups, out)
	if len(backups) == 0 {
		return "", nil
	}

	fmt.Fprintln(out, "復元する番号を入力してください。空行で中止します。")
	reader := bufio.NewReader(in)
	for {
		fmt.Fprint(out, "restore> ")
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("番号入力を読み取れませんでした: %w", err)
		}

		input := strings.TrimSpace(line)
		if input == "" {
			return "", nil
		}
		if !isPositiveInt(input) {
			fmt.Fprintf(out, "1 から %d の番号を入力してください。\n", len(backups))
			if errors.Is(err, io.EOF) {
				return "", nil
			}
			continue
		}

		index, _ := strconv.Atoi(input)
		if index < 1 || index > len(backups) {
			fmt.Fprintf(out, "1 から %d の番号を入力してください。\n", len(backups))
			if errors.Is(err, io.EOF) {
				return "", nil
			}
			continue
		}
		return backups[index-1].Path, nil
	}
}

func resolveRestorePath(dataFile, restoreArg string) (string, error) {
	restoreArg = strings.TrimSpace(restoreArg)
	if restoreArg == "" {
		return "", errors.New("復元元が指定されていません")
	}
	if !isPositiveInt(restoreArg) {
		return filepath.Clean(restoreArg), nil
	}

	backups, err := listBackupInfos(dataFile)
	if err != nil {
		return "", err
	}
	if len(backups) == 0 {
		return "", errors.New("復元できるバックアップがありません")
	}

	index, _ := strconv.Atoi(restoreArg)
	if index < 1 || index > len(backups) {
		return "", fmt.Errorf("バックアップ番号は 1 から %d の範囲で指定してください", len(backups))
	}
	return backups[index-1].Path, nil
}

func restoreEntries(dataFile string, currentEntries []Entry, restorePath string) (string, int, error) {
	restorePath = filepath.Clean(strings.TrimSpace(restorePath))
	if restorePath == "" {
		return "", 0, errors.New("復元元のバックアップファイルが空です")
	}
	if _, err := os.Stat(restorePath); err != nil {
		return "", 0, err
	}

	restoredEntries, err := loadEntries(restorePath)
	if err != nil {
		return "", 0, err
	}

	safetyBackup, err := backupEntries(currentEntries, dataFile, "")
	if err != nil {
		return "", 0, err
	}

	if err := saveEntries(dataFile, restoredEntries); err != nil {
		return "", 0, err
	}

	return safetyBackup, len(restoredEntries), nil
}

func listBackupInfos(dataFile string) ([]BackupInfo, error) {
	dir, err := defaultBackupDirPath()
	if err != nil {
		return nil, err
	}

	matches, err := filepath.Glob(filepath.Join(dir, backupFileGlob(dataFile)))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}

	backups := make([]BackupInfo, 0, len(matches))
	for _, path := range matches {
		entries, err := loadEntries(path)
		if err != nil {
			return nil, err
		}
		timestamp, err := backupTimestamp(path)
		if err != nil {
			return nil, err
		}
		backups = append(backups, BackupInfo{
			Path:      path,
			Timestamp: timestamp,
			Count:     len(entries),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		if backups[i].Timestamp.Equal(backups[j].Timestamp) {
			return backups[i].Path > backups[j].Path
		}
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})
	for i := range backups {
		backups[i].Index = i + 1
	}
	return backups, nil
}

func runList(entries []Entry, opts Options) {
	if len(entries) == 0 {
		fmt.Println("日記はまだありません。")
		return
	}

	filtered := collectEntries(entries, opts)
	if len(filtered) == 0 {
		fmt.Println(emptyMessage(opts))
		return
	}

	selected := limitEntries(filtered, opts)
	printEntries(selected, opts)
}

func runInteractiveSearch(entries []Entry, opts Options) error {
	if len(entries) == 0 {
		fmt.Println("日記はまだありません。")
		return nil
	}

	fmt.Println("対話検索モードです。検索語を入力してください。空行で終了します。")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("search> ")
		if !scanner.Scan() {
			return scanner.Err()
		}

		query := strings.TrimSpace(scanner.Text())
		if query == "" {
			fmt.Println("終了しました。")
			return nil
		}

		current := opts
		current.Search = true
		current.SearchQuery = query
		filtered := collectEntries(entries, current)
		fmt.Printf("%d 件ヒットしました。\n", len(filtered))
		if len(filtered) == 0 {
			fmt.Println(emptyMessage(current))
			fmt.Println()
			continue
		}

		selected := limitEntries(filtered, current)
		printEntries(selected, current)
		fmt.Println()
	}
}

func collectEntries(entries []Entry, opts Options) []Entry {
	filtered := make([]Entry, 0, len(entries))
	monthPrefix := ""
	if opts.ListMonth != "" {
		monthPrefix = opts.ListMonth + "-"
	}
	query := strings.ToLower(strings.TrimSpace(opts.SearchQuery))

	for _, entry := range entries {
		if monthPrefix != "" && !strings.HasPrefix(entry.Date, monthPrefix) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(entry.Text), query) {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Date == filtered[j].Date {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].Date > filtered[j].Date
	})

	return filtered
}

func limitEntries(entries []Entry, opts Options) []Entry {
	n := resolveLimit(len(entries), opts)
	selected := make([]Entry, n)
	copy(selected, entries[:n])

	if !opts.Reverse {
		for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
			selected[i], selected[j] = selected[j], selected[i]
		}
	}

	return selected
}

func resolveLimit(total int, opts Options) int {
	if total == 0 {
		return 0
	}

	n := 7
	switch {
	case opts.Search || opts.InteractiveSearch:
		if opts.ListLimitSet {
			n = opts.ListN
		} else if !opts.List {
			n = total
		}
	case opts.ListMonth != "" && opts.List && !opts.ListLimitSet:
		n = total
	case opts.ListLimitSet:
		n = opts.ListN
	}

	if n <= 0 || n > total {
		return total
	}
	return n
}

func printEntries(entries []Entry, opts Options) {
	today := todayString()
	highlightToday := stdoutSupportsANSI()
	for _, e := range entries {
		fmt.Println(formatEntryLine(e, opts, today, highlightToday))
	}
}

func formatEntryLine(e Entry, opts Options, today string, highlightToday bool) string {
	var line string
	if opts.Numbered {
		line = fmt.Sprintf("%d  %s  %s", e.ID, e.Date, e.Text)
	} else {
		line = fmt.Sprintf("%s  %s", e.Date, e.Text)
	}
	if highlightToday && e.Date == today {
		return todayHighlightStart + line + todayHighlightEnd
	}
	return line
}

func stdoutSupportsANSI() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func emptyMessage(opts Options) string {
	switch {
	case opts.Search && opts.ListMonth != "":
		return fmt.Sprintf("%s に一致する記録は %s にありません。", opts.SearchQuery, opts.ListMonth)
	case opts.Search:
		return fmt.Sprintf("%s に一致する記録はありません。", opts.SearchQuery)
	case opts.ListMonth != "":
		return fmt.Sprintf("%s の日記はありません。", opts.ListMonth)
	default:
		return "日記はまだありません。"
	}
}

func backupEntries(entries []Entry, dataFile, backupPath string) (string, error) {
	target, err := resolveBackupPath(dataFile, backupPath)
	if err != nil {
		return "", err
	}

	copied := make([]Entry, len(entries))
	copy(copied, entries)
	if err := writeBackupFile(target, copied); err != nil {
		return "", err
	}
	if err := pruneBackupHistory(dataFile, target, maxBackupHistory); err != nil {
		return "", err
	}
	return target, nil
}

func pruneBackupHistory(dataFile, savedPath string, keep int) error {
	if keep <= 0 {
		return nil
	}

	pattern := filepath.Join(filepath.Dir(savedPath), backupFileGlob(dataFile))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(matches) <= keep {
		return nil
	}

	sort.Strings(matches)
	toRemove := len(matches) - keep
	for _, oldPath := range matches {
		if toRemove == 0 {
			break
		}
		if oldPath == savedPath {
			continue
		}
		if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		toRemove--
	}
	return nil
}

func backupFileGlob(dataFile string) string {
	baseName := strings.TrimSuffix(filepath.Base(dataFile), filepath.Ext(dataFile))
	if baseName == "" {
		baseName = "diary"
	}
	return fmt.Sprintf("%s-backup-*.jsonl", baseName)
}

func backupTimestamp(path string) (time.Time, error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	idx := strings.LastIndex(name, "-backup-")
	if idx < 0 {
		info, err := os.Stat(path)
		if err != nil {
			return time.Time{}, err
		}
		return info.ModTime(), nil
	}

	stamp := name[idx+len("-backup-"):]
	timestamp, err := time.ParseInLocation(backupTimestampLayout, stamp, time.Local)
	if err == nil {
		return timestamp, nil
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func resolveBackupPath(dataFile, backupPath string) (string, error) {
	baseName := strings.TrimSuffix(filepath.Base(dataFile), filepath.Ext(dataFile))
	if baseName == "" {
		baseName = "diary"
	}
	fileName := fmt.Sprintf("%s-backup-%s.jsonl", baseName, time.Now().Format(backupTimestampLayout))

	if strings.TrimSpace(backupPath) == "" {
		dir, err := defaultBackupDirPath()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, fileName), nil
	}

	clean := filepath.Clean(backupPath)
	info, err := os.Stat(clean)
	if err == nil && info.IsDir() {
		return filepath.Join(clean, fileName), nil
	}
	if filepath.Ext(clean) == "" {
		return filepath.Join(clean, fileName), nil
	}
	return clean, nil
}

func defaultBackupDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return platformBackupDir(runtime.GOOS, home, os.Getenv("LOCALAPPDATA")), nil
}

func platformBackupDir(goos, home, localAppData string) string {
	switch goos {
	case "windows":
		if strings.TrimSpace(localAppData) != "" {
			return filepath.Join(localAppData, "diary", "backups")
		}
		return filepath.Join(home, "AppData", "Local", "diary", "backups")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "diary", "backups")
	default:
		return filepath.Join(home, ".local", "share", "diary", "backups")
	}
}

func writeEntriesFile(path string, entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Date < entries[j].Date
	})

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := w.WriteString(string(b) + "\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}

func writeBackupFile(path string, entries []Entry) error {
	return writeEntriesFile(path, entries)
}

func nextID(entries []Entry) int {
	maxID := 0
	for _, e := range entries {
		if e.ID > maxID {
			maxID = e.ID
		}
	}
	return maxID + 1
}

func promptText() (string, error) {
	fmt.Print("本文: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return "", err
	}
	text := strings.TrimSpace(line)
	if text == "" {
		return "", errors.New("本文が空です")
	}
	return text, nil
}

func isPositiveInt(s string) bool {
	if s == "" {
		return false
	}
	n, err := strconv.Atoi(s)
	return err == nil && n > 0
}

func isDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func isYearMonth(s string) bool {
	_, err := time.Parse("2006-01", s)
	return err == nil
}

func isAddDateArg(s string) bool {
	return isDate(s) || s == "yesterday"
}

func resolveAddDateArg(s string) string {
	if s == "yesterday" {
		return yesterdayString()
	}
	return s
}

func isDeleteDateArg(s string) bool {
	return s == "today" || s == "yesterday"
}

func resolveDeleteDateArg(s string) string {
	switch s {
	case "today":
		return todayString()
	case "yesterday":
		return yesterdayString()
	default:
		return s
	}
}

func todayString() string {
	return time.Now().Format("2006-01-02")
}

func yesterdayString() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

func utf8Len(s string) int {
	return len([]rune(s))
}

func exitErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "エラー: "+format+"\n", args...)
	os.Exit(1)
}

func configFilePath() (string, error) {
	configDir, err := configDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.toml"), nil
}

func configDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return platformConfigDir(runtime.GOOS, home, os.Getenv("LOCALAPPDATA")), nil
}

func platformConfigDir(goos, home, localAppData string) string {
	switch goos {
	case "windows":
		if strings.TrimSpace(localAppData) != "" {
			return filepath.Join(localAppData, "diary")
		}
		return filepath.Join(home, "AppData", "Local", "diary")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "diary")
	default:
		return filepath.Join(home, ".config", "diary")
	}
}

func defaultDataFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "diary", "diary.jsonl"), nil
}

func defaultConfig() (Config, error) {
	dataFile, err := defaultDataFilePath()
	if err != nil {
		return Config{}, err
	}
	configDir, err := configDirPath()
	if err != nil {
		return Config{}, err
	}
	return Config{
		DataFile:              dataFile,
		MaxLen:                200,
		GoogleCalendarID:      "primary",
		GoogleCredentialsFile: filepath.Join(configDir, "google_credentials.json"),
		GoogleTokenFile:       filepath.Join(configDir, "google_token.json"),
	}, nil
}

func expandHomePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func normalizeDataFile(path string) string {
	clean := filepath.Clean(path)
	if strings.EqualFold(filepath.Ext(clean), ".jsonl") {
		return clean
	}
	if filepath.Ext(clean) == "" {
		return filepath.Join(clean, "diary.jsonl")
	}
	return clean
}

func loadConfig() (Config, error) {
	cfgPath, err := configFilePath()
	if err != nil {
		return Config{}, err
	}

	cfg, err := defaultConfig()
	if err != nil {
		return Config{}, err
	}

	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return Config{}, err
		}
		cfg.DataFile = normalizeDataFile(cfg.DataFile)
		if err := os.MkdirAll(filepath.Dir(cfg.DataFile), 0o755); err != nil {
			return Config{}, err
		}

		b, err := toml.Marshal(cfg)
		if err != nil {
			return Config{}, err
		}
		if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return Config{}, err
	}
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}

	if cfg.DataFile == "" {
		cfg.DataFile, err = defaultDataFilePath()
		if err != nil {
			return Config{}, err
		}
	}
	if cfg.MaxLen <= 0 {
		cfg.MaxLen = 200
	}
	if strings.TrimSpace(cfg.GoogleCalendarID) == "" {
		cfg.GoogleCalendarID = "primary"
	}
	defaults, err := defaultConfig()
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.GoogleCredentialsFile) == "" {
		cfg.GoogleCredentialsFile = defaults.GoogleCredentialsFile
	}
	if strings.TrimSpace(cfg.GoogleTokenFile) == "" {
		cfg.GoogleTokenFile = defaults.GoogleTokenFile
	}

	cfg.DataFile = normalizeDataFile(cfg.DataFile)

	info, err := os.Stat(cfg.DataFile)
	if err == nil && info.IsDir() {
		cfg.DataFile = filepath.Join(cfg.DataFile, "diary.jsonl")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DataFile), 0o755); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
