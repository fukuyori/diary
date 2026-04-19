# diary

A simple one-line diary application for the command line.

[日本語版 README](README.ja.md)

Current version: `0.9.3`

`diary` is a lightweight CLI tool written in Go for keeping short daily notes in JSONL format.  
Each entry is assigned a serial ID, only one entry is stored per date, and existing entries can be updated or deleted easily.

---

## What's New in 0.9.3

- Added append mode for existing entries with `-A`
- Added listing by year and month with `-m YYYY-MM`
- Added case-insensitive text search with `-s`
- Added interactive search mode with `-i`
- Added automatic backup on add, update, and delete
- Added manual backup with `-b`
- Added restore from backup with `-R`
- Added restore confirmation that requires typing `diary`

---

## Features

- Simple command-line interface
- Store diary entries in JSONL format
- One entry per date
- Automatic serial ID assignment
- Update an existing entry by writing to the same date
- Append to an existing entry for the same date
- List recent entries
- List entries for a specific month
- Search entries case-insensitively
- Interactive narrowing search
- Show entries in oldest-first or newest-first order
- Optionally display serial IDs
- Delete entries by serial ID
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
````

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

### Show version

```bash
diary -v
```

### Add an entry for today

```bash
diary -a "A quiet day."
```

### Add or update an entry for a specific date

```bash
diary -a 2026-03-25 "Went for a walk."
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

---

## Command Summary

| Command                      | Description                                               |
| ---------------------------- | --------------------------------------------------------- |
| `diary`                      | Show help                                                 |
| `diary -v`                   | Show version                                              |
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
| `diary -d ID`                | Delete an entry by serial ID                              |
| `diary -b [path]`            | Create a backup immediately                               |
| `diary -R`                   | List available backups and prompt for a restore number    |
| `diary -R backup.jsonl`      | Restore from a backup file                                |

---

## Behavior

* Only one entry is stored per date.
* Adding a new entry for an existing date updates the previous one.
* Serial IDs are assigned only when a new entry is first created.
* Updating an existing entry keeps its original serial ID.
* Deletion is performed by serial ID.
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
