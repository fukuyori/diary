# Changelog

All notable changes to this project will be documented in this file.

## [2.0.2] - 2026-07-25

### Changed

- The help shown by running `diary` without arguments now displays the current environment's configuration file path and contents.
- Removed the static configuration example from the help output.

## [2.0.1] - 2026-07-24

### Changed

- Changed the default number of entries shown by `diary -l` from 7 to 20.

## [2.0.0] - 2026-06-17

### Added

- Added `diary --sync-google [YYYY-MM]` to sync diary items to Google Calendar.
- Added Google OAuth configuration support.

### Changed

- Google Calendar sync creates one all-day event for each slash-separated diary item.
- macOS config files now use `~/Library/Application Support/diary/config.toml`.
- List output such as `diary -l` highlights today's entry when printed directly to a terminal.

### Fixed

- Includes the Linux `diary -v` calendar HTML output fix from 1.0.2.

### Documentation

- Added Google Calendar OAuth setup instructions.
- Updated first-run sync documentation for the local OAuth callback flow.

## [1.0.2] - 2026-06-07

### Fixed

- Fixed `diary -v` on Linux by writing calendar HTML to a browser-accessible user directory.

## [1.0.1] - 2026-06-04

### Added

- Added `diary --sync-google [YYYY-MM]` to create missing monthly diary item events in Google Calendar.

### Changed

- List output such as `diary -l` now highlights today's entry when printed directly to a terminal.
- Google Calendar sync now creates one all-day event for each slash-separated diary item.

## [1.0.0] - 2026-06-04

### Added

- Added a browser-based calendar GUI with `diary -v`.
- Added `diary -v YYYY-MM` to show a specified month in the calendar GUI.

### Changed

- Moved version output from `-v` to `--version`.
- Calendar entries now display slash-separated items on separate lines.
- Sunday and Saturday dates are color-coded in the calendar.

## [0.9.5] - 2026-06-04

### Added

- Added `yesterday` for `-a` and `-A` to operate on yesterday's entry.
- Added `today` and `yesterday` for `-d` to delete today's or yesterday's entry.
- Added `today` and `yesterday` support for `-d` item deletion, such as `diary -d today 2`.

### Documentation

- Updated the help text and README files to describe `today` and `yesterday`.

## [0.9.4] - 2026-06-04

### Added

- Added `diary -d ID n` to delete only the nth slash-separated item from an entry.
- The item number is 1-based, and out-of-range item deletion leaves the data unchanged.

### Documentation

- Updated the help text and README files to describe the expanded `-d` behavior.

## [0.9.3] - 2026-04-19

### Added

- Added `-A` to append text to an existing entry for the same date.
- Appending uses `" / "` as the separator when an entry already exists.
- If no entry exists for the target date, `-A` creates a new entry with the given text.

### Documentation

- Updated the help text and README files to describe `-A`.

## [0.9.2] - 2026-04-15

### Added

- Added listing by year and month with `-m YYYY-MM`.
- Added case-insensitive search with `-s`.
- Added interactive narrowing search with `-i`.
- Added automatic backups on add, update, and delete.
- Added manual backup creation with `-b`.
- Added restore from backup with `-R`.
- Added restore confirmation that requires typing `diary`.

## [0.9.1] - 2026-04-13

### Added

- Added a Japanese README.
- Added prebuilt release archives for multiple platforms.

### Changed

- Refined project documentation and packaging for release use.

## [0.9.0] - 2026-03-26

### Added

- Initial release.
- Added one-line diary entry storage in JSONL format.
- Added per-date add or update behavior with `-a`.
- Added recent entry listing with `-l`.
- Added reverse listing with `-r`.
- Added numbered listing with `-n`.
- Added entry deletion by serial ID with `-d`.
- Added TOML-based configuration with configurable data path and max length.
