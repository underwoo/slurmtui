# slurmtui

A terminal interface for viewing Slurm jobs and their details.

## Requirements

- Slurm **25.11 or newer**, with JSON output available from `squeue`, `sacct`, and `scontrol`.
- At runtime, these Slurm commands must be on `PATH` and configured to access the cluster:
  - `squeue --json`
  - `sacct --json`
  - `scontrol show job --json`
- Go **1.26 or newer** to build from source.

slurmtui uses Slurm's JSON interfaces; it has no plain-text output fallback.

## Download

Tagged semantic versions publish static Linux binaries for amd64 and arm64, along with a `SHA256SUMS` file, on the [GitHub Releases page](https://github.com/underwoo/slurmtui/releases). The repository is private, so authenticate the GitHub CLI first with `gh auth login`.

For example, download the `v0.1.0` release and verify the assets:

```sh
gh release download v0.1.0 --repo underwoo/slurmtui
sha256sum -c SHA256SUMS
```

Run the binary for your machine's architecture:

```sh
./slurmtui-linux-amd64
# or
./slurmtui-linux-arm64
```

The binaries are attached directly to the release; no Go toolchain is needed on the machine where you run them.

## Build and run

```sh
git clone https://github.com/underwoo/slurmtui.git
cd slurmtui
make build
./slurmtui
```

`make build` writes the executable to `./slurmtui`. To print its build version, run:

```sh
./slurmtui --version
```

If Go is unavailable on the machine where you will run slurmtui, build for Linux amd64 on another machine with Go installed, then copy the binary to the Slurm environment:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build
```

The binary still needs the Slurm commands and cluster access described above at runtime.

## Views and keyboard shortcuts

The **Queue** view shows active jobs. **Recent** shows jobs that completed within the configured recent window. Use the arrow keys or `j`/`k` to move through the list.

| Key | Action |
| --- | --- |
| `Tab`, `1`, `2` | Switch between Queue and Recent (`1` for Queue, `2` for Recent) |
| `↑`/`↓`, `j`/`k` | Select a job |
| `PgUp`/`PgDn` | Move through the list or scroll job details |
| `Enter` | Open details for the selected job |
| `Esc` | Return from job details to the list |
| `a` | Show or hide the raw All fields section in job details |
| `r` | Refresh the job lists |
| `u` | Toggle between your jobs and all visible users' jobs |
| `?` | Show or hide the help line |
| `q`, `Ctrl+C` | Quit |

Job details include summary, requested and allocated resources, timing, paths, accounting steps when available, and reason/dependency information. Active jobs are queried with `scontrol`; Recent jobs use `sacct`.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--refresh <duration>` | Queue refresh interval | `15s` |
| `--window <duration>` | How far back Recent includes completed jobs | `1h` |
| `--all` | Start with all visible users' jobs | Your jobs (`me`) |
| `--version` | Print the build version and exit | — |

Durations use Go duration syntax, such as `30s`, `5m`, or `2h`. Examples:

```sh
./slurmtui --refresh 30s
./slurmtui --window 12h
./slurmtui --all
```

The `u` key changes the current scope while the program is running.

## Configuration

Configuration is read from `slurmtui/config.toml` under `$XDG_CONFIG_HOME`. If `XDG_CONFIG_HOME` is unset, slurmtui uses the platform's user config directory (on Linux, usually `~/.config`). For example, the Linux default path is `~/.config/slurmtui/config.toml`.

```toml
refresh_interval = "15s"
recent_window = "1h"
default_scope = "me"
```

`default_scope` can be `"me"` or `"all"`. If the file is missing, these defaults apply. Invalid values produce an error at startup. Command-line options override the file; `--all` selects the all-users scope.
