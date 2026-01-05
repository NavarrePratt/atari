# TUI Keyboard Shortcuts

This document lists all keyboard shortcuts available in the atari TUI.

## Global Keys

These keys work regardless of which pane is focused.

| Key | Action |
|-----|--------|
| `ctrl+c` | Quit (or cancel query if observer is loading) |
| `tab` | Cycle focus between open panes |
| `esc` | Exit fullscreen, clear input, or close pane |
| `e` | Toggle events pane |
| `o` | Toggle observer pane |
| `b` | Toggle graph (bead) pane |
| `E` | Toggle events fullscreen |
| `O` | Toggle observer fullscreen |
| `B` | Toggle graph fullscreen |

## Control Keys

These keys are blocked when the observer pane is focused (to allow typing).

| Key | Action |
|-----|--------|
| `q` | Quit |
| `p` | Pause drain |
| `r` | Resume drain |

## Events Pane

| Key | Action |
|-----|--------|
| `up`, `k` | Scroll up |
| `down`, `j` | Scroll down |
| `home`, `g` | Scroll to top |
| `end`, `G` | Scroll to bottom |

## Observer Pane

The observer pane uses vim-style modes.

### Normal Mode

| Key | Action |
|-----|--------|
| `i` | Enter insert mode |
| `enter` | Submit question |
| `ctrl+c` | Cancel in-progress query |
| `esc` | Clear error, clear input, or close pane |
| `up`, `k` | Scroll history up |
| `down`, `j` | Scroll history down |
| `pgup` | Half page up |
| `pgdown` | Half page down |
| `home`, `g` | Scroll to top |
| `end`, `G` | Scroll to bottom |

### Insert Mode

| Key | Action |
|-----|--------|
| `esc` | Exit insert mode |
| `enter` | Submit question |
| `ctrl+c` | Cancel in-progress query |

## Graph Pane

| Key | Action |
|-----|--------|
| `up`, `k` | Navigate to parent node |
| `down`, `j` | Navigate to child node |
| `left`, `h` | Navigate to previous sibling |
| `right`, `l` | Navigate to next sibling |
| `a` | Toggle Active/Backlog view |
| `c` | Collapse/expand selected epic |
| `d` | Cycle density level |
| `R` | Refresh graph data |
| `enter` | Open detail modal |
| `esc` | Clear error or close pane |

## Detail Modal

| Key | Action |
|-----|--------|
| `esc`, `enter`, `q` | Close modal |
| `up`, `k` | Scroll up |
| `down`, `j` | Scroll down |
| `home`, `g` | Scroll to top |
| `end`, `G` | Scroll to bottom |
