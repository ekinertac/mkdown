# VHS tape for the README demo. Regenerate with:  vhs demo/mkdown.tape
# (run from the repo root so the examples/ paths resolve).
# Requires: vhs, ffmpeg, ttyd, and mkdown on PATH.
#
# Keep this short and action-first: one file, then a whole folder in parallel,
# then proof of the generated pages. The batch timing line is the hook.

Output demo/mkdown.gif

Require mkdown

Set Shell "bash"
Set FontSize 22
Set Width 1180
Set Height 640
Set Padding 28
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms
Set Framerate 24

# Clean, minimal prompt — hide the setup from the recording.
Hide
Type "export PS1='\$ '"
Enter
Type "clear"
Enter
Show

Sleep 600ms
Type "# one markdown file -> one self-contained HTML page"
Sleep 400ms
Enter
Type "mkdown examples/showcase.md"
Sleep 400ms
Enter
Sleep 1500ms

Type "# now a whole folder, converted in parallel"
Sleep 400ms
Enter
Type "mkdown examples/*.md"
Sleep 400ms
Enter
Sleep 1600ms

Type "ls examples/*.html"
Enter
Sleep 3s
