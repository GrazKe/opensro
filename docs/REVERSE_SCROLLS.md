# Reverse return scrolls

Double-click a reverse return scroll in the bag. The confirmation window offers
the last recall location, the last death location, and (when enabled) map selection.
The recorded locations already persist with the character's world state.

Map selection opens the existing world map. Click a gold hunting-point icon and
confirm Yes to travel. No returns to map selection; closing the map cancels.
The map has no additional Cancel button or blue destination label rectangles.
Five towns use exactly the existing recall/rebirth gate positions. Other points
come from outdoor unique monster nest anchors, independent of live monster spawns.
The current v1.150 catalog produces 75 distinct destinations.

Opening or cancelling does not consume anything. The server checks the bag slot,
item identity, destination, life state, and existing return restrictions before
consuming one scroll and starting the item-data cast (one second for the retail
reverse scroll). Duplicate requests during the cast are refused. Normal return
casting, interruption and world reentry remain owned by the existing return lane.

## Native evidence and extension

The v1.150 item family is 3/3/3/3. Its itemdata duration and textuisystem question,
last-recall and last-death strings are reused. The bag wire choice tail is an
inference, documented in reverse_scroll.go; no new verified disassembly claim
is made. The prior NPC gate/guide reverse-return flow remains available.

The three-option map reference is from a later-version interface. Its point list
is inferred from the existing catalog and marked port-only, not native. Both
settings default off for upstream contribution:

- Server: set SRO_REVERSE_MAP=1 before deploying GameWorld. The Nomad deployer
  passes it into the job; the public reference catalog exposes bounded point IDs.
- Client: Escape -> Experimental -> Developer -> Reverse scroll map -> Confirm.
  This preference is saved in the browser.

The local start batch file and dev-env.ps1 enable the server flag for this user's
requested configuration. These local launch settings are not part of the commit.
The client preference must be confirmed once in each browser profile.

Clients submit a point ID, never coordinates. Invalid IDs, missing recorded points,
malformed choices, disabled map mode and refused uses preserve the scroll stack.
All client work is in existing files; no client source or test files were added.

## Validation

Behaviour tests cover both recorded choices, malformed/missing choices, duplicate
uses, one-scroll consumption, cast completion, both map flag settings, town spawn
agreement, outdoor unique filtering, catalog validation and map projection.
The UI regression test covers the compact prompt, experimental opt-in, map points,
No without an item-use request, and Yes dispatching the selected point ID.

The Windows source gate requires a CGO C compiler for its race step. Existing
full-client failures in stall admission and the Constantinople draw budget are
reported separately from reverse-scroll behaviour tests.
