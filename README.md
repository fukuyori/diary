# diary

A simple one-line diary application for the command line.

[日本語版 README](README.ja.md)

Current version: `1.0.0`

[Changelog](CHANGELOG.md)

`diary` is a lightweight CLI tool written in Go for keeping short daily notes in JSONL format.  
Each entry is assigned a serial ID, only one entry is stored per date, and existing entries can be updated or deleted easily.

---

## What's New in 1.0.0

- Changed `diary -v` to open a browser calendar GUI
- Added `diary -v YYYY-MM` to show a specified month
- Calendar entries display `/` separated items on separate lines
- Weekend dates are color-coded in the calendar

---

## Features

- Simple command-line interface
- Store diary entries in JSONL format
- One entry per date
- Automatic serial ID assignment
- Update an existing entry by writing to the same date
- Append to an existing entry for the same date
- Add or append to yesterday's entry with `yesterday`
- List recent entries
- List entries for a specific month
- Search entries case-insensitively
- Interactive narrowing search
- Show entries in oldest-first or newest-first order
- Optionally display serial IDs
- Delete entries by serial ID
- Delete today's or yesterday's entry with `today` / `yesterday`
- Delete one slash-separated item from an entry
- Automatic backup on write
- Manual backup and restore
- TOML-based configuration

---

## Data Format

Entries are stored as JSON Lines (`.jsonl`), one record per line.

Example:

```json
{"id":1,"date":"2026-03-25","text":"Went for a walk.","created_at":"2026-03-25T21:00:00+09:00","updated_at":"2026-03-25T21:00:00+09:00"}
{"id":2,"date":"2026-03-26","text":"A quiet day.","created_at":"2026-03-26T22:00:00+09:00","updated_at":"2026-03-26T22:15:00+09:00"}
```

---

## Installation

### Requirements

* Go 1.21 or later

### Build

```bash
go mod init diary
go get github.com/pelletier/go-toml/v2
go build -o diary .
```

On Windows:

```bash
go build -o diary.exe .
```

---

## Configuration

The application uses a TOML configuration file:

```text
~/.config/diary/config.toml
```

Example:

```toml
data_file = "C:\\Users\\yourname\\diary\\diary.jsonl"
max_len = 200
```

### Options

* `data_file`: path to the JSONL data file
* `max_len`: maximum number of characters allowed in one entry

---

## Usage

### Show help

```bash
diary
```

### Show a calendar in a GUI

```bash
diary -v
diary -v 2026-03
```

This opens a browser calendar for the current or specified month and shows registered diary text by date.

### Show version

```bash
diary --version
```

### Add an entry for today

```bash
diary -a "A quiet day."
```

### Add or update an entry for a specific date

```bash
diary -a 2026-03-25 "Went for a walk."
```

### Add or update yesterday's entry

```bash
diary -a yesterday "Went for a walk."
```

### Append to today's entry

```bash
diary -A "Play"
```

If an entry already exists, the result becomes `"existing text / Play"`. If no entry exists yet, it simply saves `Play`.

### Append to an entry for a specific date

```bash
diary -A 2026-03-25 "Play"
```

### Append to yesterday's entry

```bash
diary -A yesterday "Play"
```

### List the most recent 7 entries in oldest-first order

```bash
diary -l
```

### List the most recent 30 entries in oldest-first order

```bash
diary -l 30
```

### List the most recent 30 entries in newest-first order

```bash
diary -r -l 30
```

### List all entries for a specific year and month

```bash
diary -m 2026-03 -l
```

### List entries for a specific year and month in newest-first order

```bash
diary -m 2026-03 -r -l
```

### Search entries case-insensitively

```bash
diary -s "walk"
```

### Search entries in a specific month

```bash
diary -m 2026-03 -s "walk"
```

### Start interactive search mode

```bash
diary -i
```

### Create a backup immediately

```bash
diary -b
```

### Create a backup in a specific directory

```bash
diary -b backups
```

### Restore from a backup file

```bash
diary -R C:\path\to\diary-backup-20260413-164441-000000000.jsonl
```

This command first creates a safety backup of the current data.

It then asks you to type `diary` before it restores.

### List available backups

```bash
diary -R
```

This shows numbered backups with timestamp and record count, then asks which number to restore without returning to the command line.

### List entries with serial IDs

```bash
diary -n -l 30
```

### List entries with serial IDs in newest-first order

```bash
diary -r -n -l 30
```

### Delete an entry by serial ID

```bash
diary -d 3
```

### Delete today's or yesterday's entry

```bash
diary -d today
diary -d yesterday
```

### Delete one slash-separated item

```bash
diary -d 101 2
diary -d today 2
diary -d yesterday 2
```

If the entry text is `a / b / c`, this removes only the second item and leaves `a / c`.
Item numbers are 1-based. If the specified item does not exist, the data is left unchanged and an error is shown.

---

## Command Summary

| Command                      | Description                                               |
| ---------------------------- | --------------------------------------------------------- |
| `diary`                      | Show help                                                 |
| `diary -v [YYYY-MM]`         | Show a calendar for the current or specified month in a GUI |
| `diary --version`            | Show version                                              |
| `diary -l [n]`               | List recent entries in oldest-first order                 |
| `diary -m YYYY-MM -l [n]`    | List entries for the specified year and month             |
| `diary -s "query"`           | Search entries case-insensitively                         |
| `diary -m YYYY-MM -s "query"`| Search entries in the specified month                     |
| `diary -i`                   | Start interactive search mode                             |
| `diary -r -l [n]`            | List recent entries in newest-first order                 |
| `diary -n -l [n]`            | List recent entries with serial IDs                       |
| `diary -r -n -l [n]`         | List recent entries with serial IDs in newest-first order |
| `diary -a "text"`            | Add or update today's entry                               |
| `diary -a YYYY-MM-DD "text"` | Add or update an entry for a specific date                |
| `diary -a yesterday "text"`  | Add or update yesterday's entry                           |
| `diary -A "text"`            | Append to today's entry, or create it if missing          |
| `diary -A YYYY-MM-DD "text"` | Append to an entry for a specific date, or create it      |
| `diary -A yesterday "text"`  | Append to yesterday's entry, or create it if missing      |
| `diary -d ID`                | Delete an entry by serial ID                              |
| `diary -d today`             | Delete today's entry                                      |
| `diary -d yesterday`         | Delete yesterday's entry                                  |
| `diary -d ID n`              | Delete the nth slash-separated item from an entry         |
| `diary -d today n`           | Delete the nth slash-separated item from today's entry    |
| `diary -d yesterday n`       | Delete the nth slash-separated item from yesterday's entry |
| `diary -b [path]`            | Create a backup immediately                               |
| `diary -R`                   | List available backups and prompt for a restore number    |
| `diary -R backup.jsonl`      | Restore from a backup file                                |

---

## Behavior

* Only one entry is stored per date.
* Adding a new entry for an existing date updates the previous one.
* `diary -a yesterday "text"` adds or updates yesterday's entry.
* `diary -A yesterday "text"` appends to yesterday's entry.
* Serial IDs are assigned only when a new entry is first created.
* Updating an existing entry keeps its original serial ID.
* Deletion is performed by serial ID.
* `diary -d today` and `diary -d yesterday` delete today's or yesterday's entry.
* `diary -d ID n` deletes only the nth item after splitting the entry text by `/`. n is 1-based.
* `diary -d today n` and `diary -d yesterday n` delete only the nth item from today's or yesterday's entry.
* If the item specified by `diary -d ID n`, `diary -d today n`, or `diary -d yesterday n` does not exist, the data is left unchanged.
* Text search is case-insensitive.
* `-i` starts a prompt-based narrowing search loop and exits on an empty line.
* Add, update, and delete automatically create a timestamped `.jsonl` backup.
* Backups are kept up to 10 files per diary data file, with older ones removed first.
* Automatic backups are stored in an OS-local directory.
* Windows: `%LOCALAPPDATA%\diary\backups`
* Linux: `~/.local/share/diary/backups`
* macOS: `~/Library/Application Support/diary/backups`
* `-b` creates an immediate backup in the same default location unless a path is given.
* `-R` with no argument lists numbered backups with timestamp and record count, then asks for the number to restore.
* `-R backup.jsonl` restores from a backup file, first saves the current data as a safety backup, and requires typing `diary` to proceed.

---

## Project Goals

This project aims to be:

* small
* readable
* easy to build
* easy to back up
* easy to manage with Git

---

## License

This project is licensed under the MIT License.
See the `LICENCE` file for details.
