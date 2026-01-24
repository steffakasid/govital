# Govital Configuration Guide

## Overview

Govital supports flexible configuration through multiple methods with a clear priority system. Configuration determines how dependencies are classified as "healthy" (active) or "stale" (inactive).

## Configuration Options

### Scanner Configuration

#### `stale_threshold_days`
- **Description**: Number of days a dependency can be inactive before being marked as stale
- **Type**: Integer
- **Default**: 365 (1 year)
- **Recommendation**:
  - 30 days: Very aggressive (expect many false positives for mature libraries)
  - 90 days: Strict (good for fast-moving projects)
  - 180 days: Moderate (balanced for most projects)
  - 365 days: Lenient (accepts mature, stable libraries)
  - 730 days: Very lenient (only flags abandoned projects)

#### `active_threshold_days`
- **Description**: Number of days a dependency must have been updated within to be considered actively maintained
- **Type**: Integer
- **Default**: 90 (3 months)
- **Note**: Currently a placeholder for future enhancement (not actively used yet)

#### `log_level`
- **Description**: Logging verbosity
- **Type**: String
- **Default**: `info`
- **Options**: `debug`, `info`, `warn`, `error`

## Configuration Methods

### 1. CLI Flags (Highest Priority)

Override any setting directly on the command line:

```bash
# Override stale threshold
govital scan --stale-threshold 180

# Override log level
govital scan --log-level debug

# Combine with other flags
govital scan --project-path /path/to/project --stale-threshold 90 --log-level debug
```

Available flags:
- `-t, --stale-threshold int`: Days before marking as stale (default 365)
- `-p, --project-path string`: Path to scan (default ".")
- `-l, --log-level string`: Logging level (default "info")

### 2. Configuration File

Create `govital.yaml` in one of these locations (checked in order):

1. Current directory (`.`)
2. `$HOME/.govital/`
3. `/etc/govital/`

Example configuration:

```yaml
# Log level
log_level: info

# Scanner settings
scanner:
  # Days before marking dependency as stale
  stale_threshold_days: 365
  
  # Days for active maintenance threshold
  active_threshold_days: 90
```

### 3. Environment Variables

Future support planned. Currently not implemented but reserved for:
- `GOVITAL_STALE_THRESHOLD_DAYS`
- `GOVITAL_ACTIVE_THRESHOLD_DAYS`
- `GOVITAL_LOG_LEVEL`

## Configuration Priority (Highest to Lowest)

1. **CLI Flags** - Use these to override for one-off scans
2. **Config File** - Use for project-level or user-level defaults
3. **Environment Variables** - Future support
4. **Built-in Defaults** - Fallback values

## Examples

### Strict Project Policy (No Stale Dependencies)

```yaml
# .govital.yaml in project root
log_level: warn

scanner:
  stale_threshold_days: 90
```

Usage:
```bash
govital scan
```

### Lenient Corporate Standard (Allows Mature Libraries)

```yaml
# /etc/govital/govital.yaml
log_level: info

scanner:
  stale_threshold_days: 730  # 2 years
```

Usage:
```bash
govital scan /path/to/project
```

### One-Time Audit with Custom Threshold

```bash
govital scan --stale-threshold 180 --log-level debug
```

### Override Config File with CLI

If `/etc/govital/govital.yaml` specifies 365 days but you want 180:

```bash
govital scan --stale-threshold 180
# The CLI flag overrides the config file setting
```

## Understanding Results

When you run `govital scan`, it reports:

```
Stale Threshold: 365 days

Summary:
  Total Dependencies:        32
  Inactive Dependencies:     16
  Errors:                    0

Dependencies:
  - github.com/user/pkg@v1.2.3 [✗ Inactive] (last commit: 404 days ago)
  - github.com/other/lib@v2.1.0 [✓ Active] (last commit: 45 days ago)
```

- **Stale Threshold**: Shows the current setting (from CLI, config file, or default)
- **✓ Active**: Last commit within threshold (e.g., < 365 days ago)
- **✗ Inactive**: Last commit exceeded threshold (e.g., > 365 days ago)
- **Days ago**: Calculated from module release date to today

## Common Use Cases

### Case 1: New Project - Strict Standards

Want to ensure all dependencies are actively maintained:

```bash
# One-time check with 90 days threshold
govital scan --stale-threshold 90
```

### Case 2: Legacy Project - Mature Dependencies

Project uses stable, mature libraries that update infrequently:

```bash
# Create ~/.govital/govital.yaml
cat > ~/.govital/govital.yaml << 'EOF'
scanner:
  stale_threshold_days: 730
EOF

govital scan
```

### Case 3: CI/CD Pipeline Audit

Run in CI with specific policy:

```bash
# In your CI pipeline
govital scan --stale-threshold 365 --log-level warn

# Check for inactive dependencies and fail if found
INACTIVE=$(govital scan --stale-threshold 365 2>/dev/null | grep "✗ Inactive" | wc -l)
if [ $INACTIVE -gt 5 ]; then
  exit 1
fi
```

## Troubleshooting

### Config file not being read

1. Verify location (current dir, `~/.govital/`, or `/etc/govital/`)
2. Check file is named `govital.yaml` (not `.yml` or other formats)
3. Run with `--log-level debug` to see config loading attempts:
   ```bash
   govital scan --log-level debug
   ```

### Unexpected threshold behavior

1. Check CLI flags override config file - verify no flags are set
2. View current settings with:
   ```bash
   govital scan --log-level debug
   ```

### All dependencies marked as inactive

1. Your threshold may be too strict
2. Try increasing threshold:
   ```bash
   govital scan --stale-threshold 365
   ```
