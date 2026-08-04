# a gdu fork

This is a fork of [gdu](https://github.com/dundee/gdu), a pretty fast disk usage
analyzer written in Go. See the
[upstream repository](https://github.com/dundee/gdu) for the full documentation:
installation options, the complete flag and configuration reference, export and
database modes, styling, and benchmarks.

## about this fork

The fork keeps its own commits as a linear series on top of upstream `master`
and adds:

- a collector panel that keeps marked items while you navigate;
- a `t` hotkey that hands items to a trash command of your choice;
- green names for Git-tracked files and directories;
- compressed-size reporting for APFS; and
- `y`/`n` shortcuts on the confirmation dialogs.

Everything else works the way upstream gdu does. The fork is rebased onto
upstream from time to time rather than continuously, so it can trail the
upstream tip by a few commits. All of the new flags can also be set in the
config file — see [configuration.md](configuration.md).

### collector panel

`--collector` turns marking into a persistent set. Marks survive directory
changes, and the collected paths get their own panel beside or below the
directory list.

    gdu --collector /some/dir                       # panel beside the list
    gdu --collector --collector-split h /some/dir   # panel below the list

`--collector-split` accepts `v` or `vertical` (the default) and `h` or
`horizontal`; any other value is rejected at startup. The panel title carries
the current count, and entries that live in the directory you are currently
looking at are highlighted.

`Tab` cycles focus through the directory list, whichever filter inputs are open,
and the collector; `Shift-Tab` cycles backwards; `Esc` returns from the
collector to the directory list. While the collector has focus, the usual
navigation keys (arrows, `hjkl`, `g`/`G`, page keys) move through the collected
paths, `Space` or `d` drops the selected entry, and `D` empties the collector.
Every other item action is ignored while the collector is focused, so a stray
keypress cannot act on the unfocused directory list.

Actions invoked from the directory list apply to the whole collection: `d`
(delete), `e` (empty), and `t` (trash) each run over every collected item, and
an item nested under another collected item is not acted on twice. `p` marks the
collector for printing, and gdu writes whatever is still collected to stdout
when it exits.

### trash hotkey

`--trash-cmd` names a command that moves items out of the way instead of
deleting them, and enables the `t` hotkey in interactive mode:

    gdu --trash-cmd trash              # t runs: trash <path>
    gdu --trash-cmd 'trash-put -v'     # t runs: trash-put -v <path>

The value is split like a shell command line and the item's path is appended as
the final argument. `t` acts immediately — there is no confirmation dialog — and
the row disappears only if the path is really gone once the command returns. A
failing command leaves the tree alone and shows its combined output in an error
modal.

Trashing follows the same rules as deleting: it is refused under `--no-delete`,
inside archives, and while a time filter is active unless
`GDU_ALLOW_DELETE_WITH_FILTER=1` is set.

### Git-tracked colors

`--git-colors` shows the names of Git-tracked files in green, both in the
interactive table and in non-interactive output. A directory is green when it is
an index entry itself (a submodule) or contains at least one tracked file:

    gdu --git-colors ~/src

Tracking is read from the index of the nearest enclosing worktree and cached per
repository, so paths outside any repository are left alone. The flag has no
effect together with `-c`/`--no-color`, and marked or ignored rows keep their
own colors.

### compressed sizes on APFS

`--stat-compressed` reports what a transparently compressed file actually
occupies instead of its logical size:

    gdu -a --stat-compressed /System/Applications

It only affects regular files carrying macOS's `UF_COMPRESSED` flag on an APFS
volume. Uncompressed files, other filesystems, and every non-Darwin platform
keep their logical sizes.

### confirmation dialogs

The delete and empty confirmations list `yes` first rather than `no`, and accept
`y` and `n` as shortcuts for their buttons. The quit-after-a-long-scan
confirmation also takes `y`/`n`, but keeps `no` as its default button.

## building

Go 1.25 or newer, per `go.mod`.

    make build     # writes ./dist/gdu

The module path is the fork's, so installing directly from it works too:

    go install github.com/mevanlc/gdu/v5/cmd/gdu@latest

[INSTALL.md](INSTALL.md) still carries upstream's package-manager routes; apart
from the `go install` line above, those install upstream gdu rather than this
fork.

## license

MIT, unchanged from upstream — see [LICENSE.md](LICENSE.md).
