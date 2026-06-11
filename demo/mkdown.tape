# VHS tape for the README demo. Regenerate with:  vhs demo/mkdown.tape
# (run from the repo root). Requires: vhs, ffmpeg, ttyd, and mkdown on PATH.
#
# The hidden setup generates 1000 markdown files (copies of the feature-rich
# examples/showcase.md); the recorded part converts all of them in parallel,
# first with syntax highlighting, then with --no-highlight to show the tradeoff.
# Those 1000 files live in a scratch notes/ dir that is gitignored and cleaned
# up at the end.

Output demo/mkdown.gif

Require mkdown

Set Shell "bash"
Set FontSize 22
Set Width 1180
Set Height 600
Set Padding 28
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms
Set Framerate 24

# Hidden setup: generate 1000 files and set a clean prompt.
Hide
Type "rm -rf notes && mkdir notes && seq 1 1000 | xargs -P8 -I{} cp examples/showcase.md notes/note-{}.md"
Enter
Sleep 3s
Type "export PS1='\$ '"
Enter
Type "clear"
Enter
Show

Sleep 600ms
Type "# how many markdown files?"
Sleep 300ms
Enter
Type "ls notes/*.md | wc -l"
Sleep 300ms
Enter
Sleep 1200ms

Type "# convert all 1000 — one process, every core"
Sleep 300ms
Enter
Type "mkdown notes/*.md"
Sleep 300ms
Enter
Sleep 1800ms

Type "# skip highlighting for max throughput"
Sleep 300ms
Enter
Type "mkdown notes/*.md --no-highlight"
Sleep 300ms
Enter
Sleep 3s

# Hidden cleanup.
Hide
Type "rm -rf notes"
Enter
