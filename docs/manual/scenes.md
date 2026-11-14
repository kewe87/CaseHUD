# Scenes

[Manual](README.md) · Previous: [Displays](displays.md) · Next: [Lighting](lighting.md)

The **Scenes** tab (area **Display**) combines three parts: **Scenes**, **Automation** and **Rotation**. It needs connected displays ("No displays connected yet" otherwise).

![Scenes tab](img/scenes-overview.png)
<!-- BILD: Scenes tab with the scene list, automation (rules) and rotation -->

## Saving and applying scenes

A scene stores the complete assignment of the displays. Scenes stay separate from the rotation and can be used in automation and rotation.

1. Set up the displays as you like ([Displays](displays.md#setting-up-a-display)).
2. In the **Scenes** tab enter a name (e.g. "Gaming") and select **Save current displays**. If the name exists, CaseHUD asks before overwriting.
3. In the list, per scene:
   - **Apply**: the displays take the scene as their normal setup. The previous setup is replaced and should be saved as a scene first.
   - **In rotation**: off = the scene is skipped by the rotation (automation only).
   - **Rename** and delete. If rules of the automation use a scene, those rules are skipped when it is deleted. "Active" and "Automation" mark the scene currently shown or used by the automation.

**Create gaming scene** creates the scene "Gaming" from the current setup: FPS and GPU on the fans, Gaming layout on wide displays. It is not in the rotation.

## Automation

The automation switches to a scene while a game runs and back afterwards.

1. Enable the switch **Turn automation on**.
2. **Add rule**. Per rule:
   - **Condition**: **a game is running** or **a specific game is running** (then **Program name of the game**, e.g. `eldenring.exe`, or **Pick a program…**).
   - **show scene**: a saved scene or **Normal setup (rotation paused)**.
3. Rules are checked top to bottom; the first match applies. Reorder with **Move up** / **Move down**, disable a rule with **Rule on/off**.
4. Without a match the rotation applies (if active) or the normal setup per display ("Otherwise:").

Below the rule list "Currently detected: …" shows the current game; **Add a rule for "{game}"** creates a rule for it directly.

### Game detection

Games are detected by FPS measurement (foreground, fullscreen; see [FPS and latency](sensors.md#fps-and-latency)). A built-in list of ordinary programs (browsers, terminals, editors, launchers) is excluded. Own exceptions: **Ignored programs** (**Ignore** on the detected program).

### Delays

Under **Delays**, against flicker on Alt-Tab:

- **Game must run for**: 1, 3, 5 or 10 s
- **Back after the game ends**: 3, 10, 30 s or 1 min

FPS detection needs about 5 s on top.

### Manual changes during automation

The banner "Automation active: scene …" shows the running rule ("Rule {n} matches"). A manual change during the automation applies to this display only and is kept after the game; the scene is not changed. The banner offers **Edit scene …** (saves the previous setup as the scene "Normal (backup)" and takes the scene for editing; afterwards save it under **Scenes** with the same name) and **Pause automation**.

## Rotation

The rotation switches between scenes when idle or cycles designs across displays. It is paused while the automation shows a scene.

1. Enable the switch **Turn rotation on**.
2. Choose the **Type**:
   - **Scenes**: switches between the saved scenes. At least two scenes with **In rotation** are needed; the order is that of the scene list.
   - **Round robin**: on each change every display advances by one design. Choose the designs under **Designs for round robin** with **Add design…**.
3. **Change every**: 5 s, 10 s, 15 s, 30 s, 1 min or 5 min.
4. **Transition**: Fade, Slide (↓ ↑ ← →), Flip, Zoom.
5. Optionally **Change one by one**: displays change one after another, top to bottom; the order is set under **Display order (top to bottom)**.
6. **Next change** triggers a change immediately.

The status shows "Running", "Paused (game running)" or "Paused – automation shows "{scene}"". A chip in the **Displays** tab ("Rotation running") leads to the scenes.
