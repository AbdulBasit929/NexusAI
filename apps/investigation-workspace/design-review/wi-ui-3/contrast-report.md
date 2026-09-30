# WI-UI-3 contrast measurements

WCAG relative-luminance ratios were calculated from `src/styles/tokens.css`.
The automated matrix in `src/styles/tokens.vitest.js` checks every listed
foreground against every adjacent surface.

## Text minima

| Token / use | Light minimum | Dark minimum | Requirement |
| --- | ---: | ---: | ---: |
| Primary text | 12.95:1 | 9.07:1 | 4.5:1 |
| Secondary text | 6.90:1 | 6.22:1 | 4.5:1 |
| Muted text | 4.61:1 | 4.85:1 | 4.5:1 |
| Accent link text | 5.60:1 | 4.91:1 | 4.5:1 |
| Filled-control text | 7.05:1 | 7.97:1 | 4.5:1 |
| Status/evidence text | 5.88:1 | 5.93:1 | 4.5:1 |

Text surfaces checked: surface 0, 1, 2, 3, Ask, and emphasis. Status/evidence
text was checked on surfaces 1, 2, and 3.

## Control boundary against every adjacent surface

| Theme | Surface 0 | Surface 1 | Surface 2 | Surface 3 | Ask | Emphasis | Requirement |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Light | 3.71:1 | 4.22:1 | 3.96:1 | 4.34:1 | 3.77:1 | 3.45:1 | 3:1 |
| Dark | 6.27:1 | 5.64:1 | 4.91:1 | 4.20:1 | 4.75:1 | 3.56:1 | 3:1 |

## Focus ring against every adjacent surface

| Theme | Surface 0 | Surface 1 | Surface 2 | Surface 3 | Ask | Emphasis | Requirement |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Light | 5.13:1 | 5.83:1 | 5.46:1 | 5.98:1 | 5.20:1 | 4.76:1 | 3:1 |
| Dark | 10.20:1 | 9.18:1 | 7.98:1 | 6.82:1 | 7.72:1 | 5.79:1 | 3:1 |
