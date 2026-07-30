# Design: Sentry
# Type: dark · dev-tools

> Sentry's website is a dark-mode-first developer tool interface that speaks the language of code editors and terminal windows. The entire aesthetic is rooted in deep purple-black backgrounds (`#1f1633`

--- Colors ---
  Deep Purple: #1f1633
  Darker Purple: #150f23
  Border Purple: #362d59
  Sentry Purple: #6a5fc1
  Muted Purple: #79628c
  Deep Violet: #422082
  Lime Green: #c2ef4e
  Coral: #ffb287
  Pink: #fa7faa
  Pure White: #ffffff
  Light Gray: #e5e7eb
  Code Yellow: #dcdcaa

--- Typography ---
  Font: Rubik

--- Components ---
  · Primary Muted Purple
  · Glass White
  · White Solid
  · Deep Violet (Select/Dropdown)
  · Text Input
    **Default on dark**: `#ffffff`, underline decoration
    **Hover**: color transitions to `#6a5fc1` (Sentry Purple)
    **Purple links**: `#6a5fc1` default, hover underline
    **Lime accent links**: `#c2ef4e` default, hover to `#6a5fc1`
    **Dark context links**: `#362d59`, hover to `#ffffff`

--- Shadows ---
  Sunken (Level -1): Inset shadow `rgba(0, 0, 0, 0.1) 0px 1px 3px inset`
  Surface (Level 1): `rgba(0, 0, 0, 0.08) 0px 2px 8px`
  Elevated (Level 2): `rgba(0, 0, 0, 0.1) 0px 10px 15px -3px`
  Prominent (Level 3): `rgba(0, 0, 0, 0.18) 0px 0.5rem 1.5rem`