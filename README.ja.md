# diary

コマンドラインで使うシンプルな一行日記アプリです。

[English README](README.md)

現在のバージョン: `2.0.2`

[変更履歴](CHANGELOG.md)

`diary` は Go で書かれた軽量な CLI ツールで、短い日々のメモを JSONL 形式で保存します。  
各記録には連番 ID が付き、1 日につき 1 件だけ保存され、既存の記録も簡単に更新や削除ができます。

---

## 2.0.2 の更新内容

- 引数なしで `diary` を実行した際のヘルプに、現在の環境の設定ファイルパスと内容を表示
- ヘルプから固定の設定例を削除

---

## 特徴

- シンプルなコマンドライン操作
- JSONL 形式で日記を保存
- 1 日につき 1 件だけ保存
- 自動で連番 ID を採番
- 同じ日付に書くと既存記録を更新
- 同じ日付の既存記録へ追記可能
- `yesterday` 指定で前日の記録を追加・追記可能
- 最近の記録を一覧表示
- 指定年月の記録を一覧表示
- 大文字小文字を区別せず検索
- 対話的な絞り込み検索
- 古い順 / 新しい順で表示可能
- 連番 ID の表示に対応
- 連番 ID で削除可能
- `today` / `yesterday` 指定で今日または前日の記録を削除可能
- "/" 区切りの項目だけを指定して削除可能
- 書き込み時の自動バックアップ
- 手動バックアップと復元
- Googleカレンダーへの月単位同期
- TOML ベースの設定

---

## データ形式

記録は JSON Lines (`.jsonl`) 形式で、1 行に 1 レコードずつ保存されます。

例:

```json
{"id":1,"date":"2026-03-25","text":"Went for a walk.","created_at":"2026-03-25T21:00:00+09:00","updated_at":"2026-03-25T21:00:00+09:00"}
{"id":2,"date":"2026-03-26","text":"A quiet day.","created_at":"2026-03-26T22:00:00+09:00","updated_at":"2026-03-26T22:15:00+09:00"}
```

---

## インストール

### 必要要件

* Go 1.21 以降

### ビルド

```bash
go build -o diary .
```

Windows の場合:

```bash
go build -o diary.exe .
```

---

## 設定

設定ファイルはOSごとのローカル設定ディレクトリにあります。

```text
macOS: ~/Library/Application Support/diary/config.toml
Linux: ~/.config/diary/config.toml
Windows: %LOCALAPPDATA%\diary\config.toml
```

例:

```toml
data_file = "C:\\Users\\yourname\\diary\\diary.jsonl"
max_len = 200
google_calendar_id = "primary"
google_credentials_file = "~/Library/Application Support/diary/google_credentials.json"
google_token_file = "~/Library/Application Support/diary/google_token.json"
```

### 設定項目

* `data_file`: JSONL データファイルのパス
* `max_len`: 1 件の本文に許可する最大文字数
* `google_calendar_id`: 同期先のGoogleカレンダーID。通常は `primary`
* `google_credentials_file`: Google Cloud Console で作成したOAuthクライアントJSONのパス
* `google_token_file`: 初回認証後に保存するOAuthトークンのパス

---

## Googleカレンダー同期の準備

Googleカレンダー同期を使う場合は、初回実行前にGoogle Cloud ConsoleでOAuthクライアントを作成します。

1. Google Cloud Consoleでプロジェクトを作成または選択します。
2. Google Calendar APIを有効化します。
3. Google Auth Platformの同意画面を設定します。
4. Google Auth Platformの「対象」で、自分のGoogleアカウントをテストユーザーに追加します。
5. Google Auth Platformの「クライアント」で、種類を「デスクトップアプリ」にしてOAuthクライアントを作成します。
6. ダウンロードしたJSONを `google_credentials_file` の場所に保存します。

macOSの例:

```bash
mkdir -p "$HOME/Library/Application Support/diary"
mv ~/Downloads/client_secret_*.json "$HOME/Library/Application Support/diary/google_credentials.json"
chmod 600 "$HOME/Library/Application Support/diary/google_credentials.json"
```

初回の `diary --sync-google` では認証URLが表示されます。ブラウザで許可すると、一時的に起動したローカルサーバーが認証結果を受け取り、`google_token_file` にトークンを保存します。
`google_token_file` は手で作る必要はありません。

個人利用の未審査アプリでは、Googleの警告画面が表示されることがあります。自分で作成したプロジェクトで、テストユーザーに自分のアカウントを追加している場合は、詳細表示から続行できます。

---

## 使い方

### ヘルプを表示

```bash
diary
```

### GUIでカレンダーを表示

```bash
diary -v
diary -v 2026-03
```

当月または指定年月のカレンダーをブラウザで開き、登録済みの日記本文を日付ごとに表示します。

### バージョンを表示

```bash
diary --version
```

### Googleカレンダーに同期

```bash
diary --sync-google
diary --sync-google 2026-03
```

当月または指定年月の日記を確認し、本文を `/` で区切った1項目ごとに終日イベントを作成します。
Googleイベントには `diary_app=diary`、`diary_date=YYYY-MM-DD`、`diary_item_key` を保存するため、同じ月を再同期しても同じ項目は重複登録しません。
初回実行時は表示されたGoogle認証URLを開き、ブラウザで許可します。

### 今日の日記を追加

```bash
diary -a "A quiet day."
```

### 指定日の記録を追加または更新

```bash
diary -a 2026-03-25 "Went for a walk."
```

### 前日の記録を追加または更新

```bash
diary -a yesterday "Went for a walk."
```

### 今日の記録に追記

```bash
diary -A "Play"
```

既存の本文がある場合は `"既存本文 / Play"` になります。未登録ならそのまま `Play` を保存します。

### 指定日の記録に追記

```bash
diary -A 2026-03-25 "Play"
```

### 前日の記録に追記

```bash
diary -A yesterday "Play"
```

### 直近 20 件を古い順で表示

```bash
diary -l
```

### 直近 30 件を古い順で表示

```bash
diary -l 30
```

### 直近 30 件を新しい順で表示

```bash
diary -r -l 30
```

### 指定年月の記録を一覧表示

```bash
diary -m 2026-03 -l
```

### 指定年月の記録を新しい順で一覧表示

```bash
diary -m 2026-03 -r -l
```

### 大文字小文字を区別せず検索

```bash
diary -s "walk"
```

### 指定年月の中で検索

```bash
diary -m 2026-03 -s "walk"
```

### 対話検索を開始

```bash
diary -i
```

### すぐにバックアップを作成

```bash
diary -b
```

### 保存先を指定してバックアップを作成

```bash
diary -b backups
```

### バックアップファイルから復元

```bash
diary -R C:\path\to\diary-backup-20260413-164441-000000000.jsonl
```

このコマンドは、まず現在データの安全用バックアップを作成します。

その後、復元前に `diary` と入力して確認します。

### 利用可能なバックアップを一覧表示して復元

```bash
diary -R
```

バックアップを番号付きで表示し、そのままコマンドラインに戻らず復元する番号の入力を求めます。

### 連番 ID 付きで一覧表示

```bash
diary -n -l 30
```

### 連番 ID 付きで新しい順に一覧表示

```bash
diary -r -n -l 30
```

### 連番 ID で削除

```bash
diary -d 3
```

### 今日または前日の記録を削除

```bash
diary -d today
diary -d yesterday
```

### "/" 区切りの項目を削除

```bash
diary -d 101 2
diary -d today 2
diary -d yesterday 2
```

本文が `a / b / c` の場合、2 番目の `b` だけを削除して `a / c` になります。
項目番号は 1 から数えます。指定した番号の項目がない場合は、データを変更せずエラーになります。

---

## コマンド一覧

| コマンド | 説明 |
| -------- | ---- |
| `diary` | ヘルプを表示 |
| `diary -v [YYYY-MM]` | GUIで当月または指定年月のカレンダーを表示 |
| `diary --version` | バージョンを表示 |
| `diary --sync-google [YYYY-MM]` | 当月または指定年月の日記項目をGoogleカレンダーに同期 |
| `diary -l [n]` | 直近の記録を古い順で表示 |
| `diary -m YYYY-MM -l [n]` | 指定した年月の記録を表示 |
| `diary -s "query"` | 大文字小文字を区別せず検索 |
| `diary -m YYYY-MM -s "query"` | 指定した年月内で検索 |
| `diary -i` | 対話検索を開始 |
| `diary -r -l [n]` | 直近の記録を新しい順で表示 |
| `diary -n -l [n]` | 直近の記録を連番 ID 付きで表示 |
| `diary -r -n -l [n]` | 直近の記録を新しい順・連番 ID 付きで表示 |
| `diary -a "text"` | 今日の日付で追加または更新 |
| `diary -a YYYY-MM-DD "text"` | 指定日で追加または更新 |
| `diary -a yesterday "text"` | 前日の日付で追加または更新 |
| `diary -A "text"` | 今日の記録に追記。未登録なら新規追加 |
| `diary -A YYYY-MM-DD "text"` | 指定日の記録に追記。未登録なら新規追加 |
| `diary -A yesterday "text"` | 前日の記録に追記。未登録なら新規追加 |
| `diary -d ID` | 連番 ID を指定して削除 |
| `diary -d today` | 今日の記録を削除 |
| `diary -d yesterday` | 前日の記録を削除 |
| `diary -d ID n` | 連番 ID の記録から "/" 区切りの n 番目の項目を削除 |
| `diary -d today n` | 今日の記録から "/" 区切りの n 番目の項目を削除 |
| `diary -d yesterday n` | 前日の記録から "/" 区切りの n 番目の項目を削除 |
| `diary -b [path]` | すぐにバックアップを作成 |
| `diary -R` | バックアップ一覧を表示し、そのまま番号入力で復元 |
| `diary -R backup.jsonl` | バックアップファイルから復元 |

---

## 動作仕様

* 1 日につき 1 件だけ保存されます。
* 同じ日付に追加すると既存の記録が更新されます。
* `diary -a yesterday "text"` は前日の日付で記録を追加または更新します。
* `diary -A yesterday "text"` は前日の記録に追記します。
* 連番 ID は新規作成時にだけ採番されます。
* 既存記録を更新しても元の連番 ID は維持されます。
* 削除は連番 ID で行います。
* `diary -d today` と `diary -d yesterday` は、それぞれ今日または前日の記録を削除します。
* `diary -d ID n` は、本文を `/` で区切った n 番目の項目だけを削除します。n は 1 から数えます。
* `diary -d today n` と `diary -d yesterday n` は、それぞれ今日または前日の記録から n 番目の項目だけを削除します。
* `diary -d ID n`、`diary -d today n`、`diary -d yesterday n` で指定した項目が存在しない場合、データは変更されません。
* テキスト検索は大文字小文字を区別しません。
* `-i` はプロンプト付きの絞り込み検索を開始し、空行で終了します。
* 追加・更新・削除時にはタイムスタンプ付きの `.jsonl` バックアップが自動作成されます。
* バックアップは日記データファイルごとに最大 10 件まで保持し、古いものから削除します。
* 自動バックアップは OS ごとのローカル保存先に保存されます。
* Windows: `%LOCALAPPDATA%\diary\backups`
* Linux: `~/.local/share/diary/backups`
* macOS: `~/Library/Application Support/diary/backups`
* `-b` は保存先省略時、既定のバックアップ保存先に即時バックアップを作成します。
* `-R` は引数なしだと日時と件数付きのバックアップ一覧を表示し、そのまま復元する番号の入力を求めます。
* `-R backup.jsonl` はバックアップファイルから復元し、実行前に現在データを安全用バックアップとして保存し、`diary` の確認入力を要求します。
* `diary -l` などの一覧表示では、端末に直接表示する場合に当日の記録を強調表示します。
* `--sync-google` は対象月のGoogleカレンダーを確認し、本文の `/` 区切り1項目ごとに未登録分だけを終日イベントとして作成します。
* 以前のバージョンで作成した日付ごとの日記イベントは、項目単位の重複判定には使われません。

---

## プロジェクトの目標

このプロジェクトは次の性質を目指しています。

* 小さい
* 読みやすい
* ビルドしやすい
* バックアップしやすい
* Git で管理しやすい

---

## ライセンス

このプロジェクトは MIT License のもとで公開されています。  
詳細は `LICENCE` ファイルを参照してください。
