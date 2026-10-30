# Record with: bash docs/demo/record.sh
# Uses the real CLI with fictional API responses; no access token is needed.
Output docs/demo/anigrass.gif

Require anigrass
Set Shell bash
Set FontSize 18
Set Width 1240
Set Height 250
Set Padding 24
# Keep commands readable while giving the cursor a softer contrast.
Set Theme { "name": "Anigrass Demo", "black": "#15161e", "red": "#f7768e", "green": "#9ece6a", "yellow": "#e0af68", "blue": "#7aa2f7", "magenta": "#bb9af7", "cyan": "#7dcfff", "white": "#e2e4e9", "brightBlack": "#414868", "brightRed": "#f7768e", "brightGreen": "#9ece6a", "brightYellow": "#e0af68", "brightBlue": "#7aa2f7", "brightMagenta": "#bb9af7", "brightCyan": "#7dcfff", "brightWhite": "#f0f1f4", "background": "#16161e", "foreground": "#e2e4e9", "selection": "#343746", "cursor": "#707787" }
Set TypingSpeed 120ms

Hide
Type "unset NO_COLOR; export TERM=xterm-256color PS1='$ '; clear"
Enter
Sleep 500ms
Show

Type "anigrass"
Sleep 500ms
Enter
Sleep 15s
