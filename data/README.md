# Data Directory

This directory contains runtime data generated during application execution.

## Contents

- **hydra.db** - SQLite database file (when using SQLite)
- **logs/** - Application log files
- **logs/hydra.log** - Current log file
- **logs/hydra-*.log.gz** - Rotated and compressed log files

## Important Notes

- This directory is **gitignored** and should not be committed to version control
- Database files in this directory are for **development only**
- For production, use external databases (PostgreSQL recommended)
- Log files are automatically rotated and cleaned up based on retention settings

## Cleanup

To reset all runtime data:

```bash
make clean-data
```

Or manually:

```bash
rm -rf data/*
```
