# WI-UI-6 contrast report

Measured from the shipped CSS token hex values with WCAG relative luminance.
Columns are primary text / secondary text / muted text / accent link / strong
control line / focus ring against each adjacent surface. Text targets 4.5:1;
control lines and focus target 3:1.

## Daylight Ledger

| Adjacent surface | Primary | Secondary | Muted | Accent | Control line | Focus |
|---|---:|---:|---:|---:|---:|---:|
| Surface 0 `#e7ece8` | 13.76 | 7.41 | 4.97 | 6.28 | 4.09 | 5.02 |
| Reading `#f6f8f5` | 15.41 | 8.30 | 5.57 | 7.03 | 4.58 | 5.62 |
| Subtle `#edf1ee` | 14.43 | 7.77 | 5.22 | 6.59 | 4.29 | 5.27 |
| Raised `#ffffff` | 16.46 | 8.86 | 5.95 | 7.51 | 4.89 | 6.01 |
| Ask `#e4eeeb` | 13.89 | 7.48 | 5.02 | 6.34 | 4.13 | 5.07 |
| Emphasis `#d7e6e1` | 12.77 | 6.88 | 4.61 | 5.83 | 3.80 | 4.66 |

Status foreground/background pairs: positive 5.86:1, caution 6.17:1,
critical 6.62:1. Surface steps are intentionally quiet and are separated by
the single line token: surface-0/reading 1.12:1, reading/subtle 1.07:1,
subtle/raised 1.14:1, ask/reading 1.11:1.

## Operations Slate

| Adjacent surface | Primary | Secondary | Muted | Accent | Control line | Focus |
|---|---:|---:|---:|---:|---:|---:|
| Surface 0 `#11191b` | 16.06 | 11.52 | 8.46 | 8.78 | 5.63 | 11.35 |
| Reading `#192326` | 14.45 | 10.37 | 7.62 | 7.90 | 5.07 | 10.22 |
| Subtle `#223033` | 12.30 | 8.82 | 6.48 | 6.72 | 4.32 | 8.70 |
| Raised `#2d3d40` | 10.21 | 7.33 | 5.38 | 5.58 | 3.58 | 7.22 |
| Ask `#1c3132` | 12.33 | 8.85 | 6.50 | 6.74 | 4.33 | 8.72 |
| Emphasis `#244346` | 9.63 | 6.90 | 5.07 | 5.26 | 3.38 | 6.81 |

Status foreground/background pairs: positive 6.74:1, caution 7.06:1,
critical 6.60:1. Surface steps are intentionally quiet and are separated by
the single line token: surface-0/reading 1.11:1, reading/subtle 1.17:1,
subtle/raised 1.20:1, ask/reading 1.17:1.

All measured text, control-boundary, focus, and status pairs pass their stated
thresholds in both themes. `src/styles/tokens.vitest.js` independently enforces
the same token matrix on every unit run.
