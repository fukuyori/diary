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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

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
	List     bool
	ListN    int
	Reverse  bool
	Numbered bool

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
		if err := saveEntries(cfg.DataFile, entries); err != nil {
			exitErr("保存エラー: %v", err)
		}
		fmt.Println("保存しました。")

	case opts.Delete:
		var deleted bool
		entries, deleted = deleteByID(entries, opts.DeleteID)
		if !deleted {
			exitErr("ID %d のデータは見つかりませんでした", opts.DeleteID)
		}
		if err := saveEntries(cfg.DataFile, entries); err != nil {
			exitErr("保存エラー: %v", err)
		}
		fmt.Printf("ID %d を削除しました。\n", opts.DeleteID)

	case opts.List:
		printList(entries, opts)

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
				i++
			}

		case "-a":
			if opts.Delete || opts.List {
				return opts, false, errors.New("-a は -d や -l と同時に使えません")
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
			if opts.Add || opts.List {
				return opts, false, errors.New("-d は -a や -l と同時に使えません")
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
		return opts, false, errors.New("-r は -l と一緒に使ってください")
	}
	if opts.Numbered && !opts.List {
		return opts, false, errors.New("-n は -l と一緒に使ってください")
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

func printList(entries []Entry, opts Options) {
	if len(entries) == 0 {
		fmt.Println("日記はまだありません。")
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Date > entries[j].Date
	})

	n := opts.ListN
	if n <= 0 {
		n = 7
	}
	if n > len(entries) {
		n = len(entries)
	}

	selected := make([]Entry, n)
	copy(selected, entries[:n])

	if !opts.Reverse {
		for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
			selected[i], selected[j] = selected[j], selected[i]
		}
	}

	for _, e := range selected {
		if opts.Numbered {
			fmt.Printf("%d  %s  %s\n", e.ID, e.Date, e.Text)
		} else {
			fmt.Printf("%s  %s\n", e.Date, e.Text)
		}
	}
}

func printHelp() {
	fmt.Println(`1行日記 CLI

使い方:
  diary
      ヘルプを表示

  diary -l [件数]
      直近の記録を古いもの順で表示
      件数省略時は 7

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
`)
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

	return os.Rename(tmpPath, path)
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
