# nmclean

Fast Go CLI tool to find and delete `node_modules` folders across projects.

## Features

- **Fast parallel scan and deletion** using Go worker pools.
- **Interactive checklist** by default (select folders, sort by size or path, confirm before deletion).
- **Dry-run preview** (`--dry-run`) to inspect disk space without deletion.
- **Direct mode** (`--yes`) to delete all found `node_modules` folders immediately.
- **Disk space report** per folder and total freed.

## Install Locally

```bash
make install
```

Make sure `$HOME/go/bin` is in your `PATH`.

Or build a local binary in `./bin/nmclean`:

```bash
make build
```

## Usage

```bash
nmclean [directory] [flags]
```

| Command | Action |
| :--- | :--- |
| `nmclean ~/dev` | Scan `~/dev` and open the interactive checklist. |
| `nmclean ~/dev --dry-run` | Show folders, sizes, and total reclaimable space. Delete nothing. |
| `nmclean ~/dev --yes` | Delete all `node_modules` folders in `~/dev` immediately. |
| `nmclean --version` | Show build version. |

### Interactive Keys

| Key | Action |
| :--- | :--- |
| `↑` / `↓` or `k` / `j` | Move cursor |
| `Space` | Toggle folder selection |
| `a` | Select all |
| `n` | Select none |
| `s` | Switch sort order (Size / Path) |
| `Enter` | Confirm and delete selected folders |
| `q` or `Esc` | Quit without deleting |

## Publish a Standalone Release

Push a version tag to GitHub. GitHub Actions and GoReleaser build and upload binaries for macOS, Linux, and Windows automatically:

```bash
git tag v0.1.0
git push origin v0.1.0
```
