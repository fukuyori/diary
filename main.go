// Copyright (c) 2026 Noriaki Fukuyori
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const appVersion = "0.9.1"

type Entry struct {
	ID        int    `json:"id"`
	Date      string `json:"date"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Config struct {
	DataFile string `toml:"data_file"`
	MaxLen   int    `toml:"max_len"`
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

	Add     bool
	AddDate string
	AddText string

	Delete   bool
	DeleteID int
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		exitErr("設定読み込みエラー: %v", err)
	}

	opts, showHelp, err := parseArgs(os.Args[1:])
	if err != nil {
		exitErr("%v", err)
	}
	if showHelp {
		printHelp()
		return
	}

	entries, err := loadEntries(cfg.DataFile)
	if err != nil {
		exitErr("データ読み込みエラー: %v", err)
	}

	switch {
	case opts.Add:
		if err := addOrUpdateEntry(&entries, opts, cfg); err != nil {
			exitErr("%v", err)
		}
		backupPath, err := saveWithAutomaticBackup(cfg.DataFile, entries)
		if err != nil {
			exitErr("保存エラー: %v", err)
		}
		fmt.Println("保存しました。")
		fmt.Printf("自動バックアップ: %s\n", backupPath)

	case opts.Delete:
		var deleted bool
		entries, deleted = deleteByID(entries, opts.DeleteID)
		if !deleted {
			exitErr("ID %d のデータは見つかりませんでした", opts.DeleteID)
		}
		backupPath, err := saveWithAutomaticBackup(cfg.DataFile, entries)
		if err != nil {
			exitErr("保存エラー: %v", err)
		}
		fmt.Printf("ID %d を削除しました。\n", opts.DeleteID)
		fmt.Printf("自動バックアップ: %s\n", backupPath)

	case opts.Backup:
		path, err := backupEntries(entries, cfg.DataFile, opts.BackupPath)
		if err != nil {
			exitErr("バックアップエラー: %v", err)
		}
		fmt.Printf("バックアップを保存しました: %s\n", path)

	case opts.Restore:
		if err := confirmRestore(os.Stdin, os.Stdout); err != nil {
			exitErr("復元エラー: %v", err)
		}
		safetyBackup, restoredCount, err := restoreEntries(cfg.DataFile, entries, opts.RestorePath)
		if err != nil {
			exitErr("復元エラー: %v", err)
		}
		fmt.Printf("復元しました: %s\n", opts.RestorePath)
		fmt.Printf("復元前バックアップ: %s\n", safetyBackup)
		fmt.Printf("復元レコード数: %d\n", restoredCount)

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
			if i+1 >= len(args) {
				return opts, false, errors.New("-R にはバックアップファイルのパスが必要です")
			}
			opts.Restore = true
			opts.RestorePath = args[i+1]
			i++

		case "-a":
			if opts.Delete || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore {
				return opts, false, errors.New("-a は一覧・検索・削除・バックアップ・復元系のオプションと同時に使えません")
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

			if len(rest) >= 2 && isDate(rest[0]) {
				opts.AddDate = rest[0]
				opts.AddText = strings.Join(rest[1:], " ")
			} else {
				opts.AddDate = todayString()
				opts.AddText = strings.Join(rest, " ")
			}

			if strings.TrimSpace(opts.AddText) == "" {
				return opts, false, errors.New("追加する本文が空です")
			}
			return opts, false, nil

		case "-d":
			if opts.Add || opts.List || opts.Search || opts.InteractiveSearch || opts.Backup || opts.Restore {
				return opts, false, errors.New("-d は追加・一覧・検索・バックアップ・復元系のオプションと同時に使えません")
			}
			opts.Delete = true

			if i+1 >= len(args) {
				return opts, false, errors.New("-d には削除対象のシリアル番号が必要です")
			}
			if !isPositiveInt(args[i+1]) {
				return opts, false, errors.New("-d には正の整数のシリアル番号を指定してください")
			}
			id, _ := strconv.Atoi(args[i+1])
			opts.DeleteID = id
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
	if opts.Backup && (opts.Add || opts.Delete) {
		return opts, false, errors.New("-b は -a や -d と同時に使えません")
	}
	if opts.Restore && (opts.List || opts.Search || opts.InteractiveSearch || opts.ListMonth != "" || opts.Reverse || opts.Numbered) {
		return opts, false, errors.New("-R は一覧・検索系のオプションと同時に使えません")
	}
	if opts.Restore && (opts.Add || opts.Delete || opts.Backup) {
		return opts, false, errors.New("-R は -a、-d、-b と同時に使えません")
	}
	if opts.Restore && strings.TrimSpace(opts.RestorePath) == "" {
		return opts, false, errors.New("復元元のバックアップファイルが空です")
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

func printHelp() {
	fmt.Printf(`1行日記 CLI v%s

使い方:
  diary
      ヘルプを表示

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

  diary -R バックアップファイル
      バックアップファイルから復元
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

  diary -a YYYY-MM-DD "本文"
      指定日で追加または上書き

  diary -d ID
      指定したシリアル番号の記録を削除

設定ファイル:
  ~/.config/diary/config.toml

設定例:
  data_file = "C:\\Users\\yourname\\diary\\diary.jsonl"
  max_len = 200
`, appVersion)
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

func saveWithAutomaticBackup(dataFile string, entries []Entry) (string, error) {
	if err := saveEntries(dataFile, entries); err != nil {
		return "", err
	}

	backupPath, err := backupEntries(entries, dataFile, "")
	if err != nil {
		return "", fmt.Errorf("保存は完了しましたが自動バックアップに失敗しました: %w", err)
	}

	return backupPath, nil
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
	for _, e := range entries {
		if opts.Numbered {
			fmt.Printf("%d  %s  %s\n", e.ID, e.Date, e.Text)
		} else {
			fmt.Printf("%s  %s\n", e.Date, e.Text)
		}
	}
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
	return target, nil
}

func resolveBackupPath(dataFile, backupPath string) (string, error) {
	baseName := strings.TrimSuffix(filepath.Base(dataFile), filepath.Ext(dataFile))
	if baseName == "" {
		baseName = "diary"
	}
	fileName := fmt.Sprintf("%s-backup-%s.jsonl", baseName, time.Now().Format("20060102-150405-000000000"))

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

func todayString() string {
	return time.Now().Format("2006-01-02")
}

func utf8Len(s string) int {
	return len([]rune(s))
}

func exitErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "エラー: "+format+"\n", args...)
	os.Exit(1)
}

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "diary", "config.toml"), nil
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
	return Config{
		DataFile: dataFile,
		MaxLen:   200,
	}, nil
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
