# diary

A simple one-line diary application for the command line.

`diary` is a lightweight CLI tool written in Go for keeping short daily notes in JSONL format.  
Each entry is assigned a serial ID, only one entry is stored per date, and existing entries can be updated or deleted easily.

---

## Features

- Simple command-line interface
- Store diary entries in JSONL format
- One entry per date
- Automatic serial ID assignment
- Update an existing entry by writing to the same date
- List recent entries
- Show entries in oldest-first or newest-first order
- Optionally display serial IDs
- Delete entries by serial ID
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

### Add an entry for today

```bash
diary -a "A quiet day."
```

### Add or update an entry for a specific date

```bash
diary -a 2026-03-25 "Went for a walk."
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
| `diary -l [n]`               | List recent entries in oldest-first order                 |
| `diary -r -l [n]`            | List recent entries in newest-first order                 |
| `diary -n -l [n]`            | List recent entries with serial IDs                       |
| `diary -r -n -l [n]`         | List recent entries with serial IDs in newest-first order |
| `diary -a "text"`            | Add or update today's entry                               |
| `diary -a YYYY-MM-DD "text"` | Add or update an entry for a specific date                |
| `diary -d ID`                | Delete an entry by serial ID                              |

---

## Behavior

* Only one entry is stored per date.
* Adding a new entry for an existing date updates the previous one.
* Serial IDs are assigned only when a new entry is first created.
* Updating an existing entry keeps its original serial ID.
* Deletion is performed by serial ID.

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
See the `LICENSE` file for details.
