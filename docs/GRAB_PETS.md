# Grab pets

Summon the pet from its inventory item, then open its Info command in the pet
command bar. The same window provides Basic Info, Inventory, and Setting.

## Using the window

- Basic Info shows the saved pet name (or No name), remaining rental time, and
  the retail no-stats message. Grab pets do not level or fight.
- Inventory holds the pet's collected items. The initial bag has 28 slots.
  Drag items between the pet bag and player inventory to transfer them. The
  existing inventory authority validates capacity and stack limits.
- Setting has Grab function ON/OFF, Grab only my items/Grab all items, and
  Gold/Equipment/Other items. Confirm saves the selection; Cancel restores
  the last saved selection. New pets initially have grabbing OFF, as in the
  provided reference. Turn it ON and Confirm to start collecting.

Pets follow independently of whether auto-grab is enabled. Following and
pickup approach use the server's terrain and wall checks. Items go into the
pet bag; gold credits the player's gold balance without occupying bag slots.
Only legally available drops can be picked: All items does not override a
different player's temporary ownership reservation. Party-shared drops honor
the existing party pickup rules. Quest-special drops are excluded.

The bag, name, and settings remain attached to the summoner item when the pet
is dismissed or the player logs out. A full pet bag disables automatic pickup;
clear space and enable grabbing again. Expiry dismisses the pet and keeps its
stored items. Lease renewal restores access to that same pet.

Summoned item icons have a yellow border and the retail animated edge. Dead
or expired item icons have a blue wash. The Experimental window has a Smaller grab pets option (off by default),
which uses 75% of their previous size. Monsters ignore grab pets; direct and area monster attacks
cannot damage them. Set `SRO_PET_PACING=1` to run grab and growth pets at 80% of their previous speed;
walking and mount speeds are unchanged.

Set `SRO_PET_RECOVERY=1` to let grab and growth pets relocate beside their owner when the separation exceeds
1,200 game units, or after 5 seconds without at least 2 units of movement
progress while trying to move. Idle pets and movement-blocking status effects
do not accrue a stuck timer. Recovery cancels the pending pickup or combat
intent, preserving the same pet, inventory, health, and lease. These thresholds
are port-only, not native. Both server flags default off, preserving the
native behaviour.

## Evidence and implementation

The provided screenshots and local v1.150 client files define the window:
`C:/SRO/extracted/Media_extracted/resinfo/ifcos.txt`, `ifcosinfo.txt`,
`ifcosinventory.txt`, and `ifcossetup.txt`. Character data classifies grab pets
as COS band 4 and supplies their movement speeds. The existing implementation
cites native command, record, filter, and slot-overlay handlers; those prior
disassembly claims were not independently re-disassembled for this change.

[Joymax's October 2007 pet announcement, reproduced by MMORPG.com](https://www.mmorpg.com/news/new-pets-2000063909)
describes ability pets collecting nearby items into their own inventory.
The packet and UI details here come from the extracted client and existing
repository contracts, rather than later private-server bot features.

The movement repair compares the terrain height at the integer precision used
by destination packets. It still revalidates the rounded horizontal endpoint
against collision, so the repair does not bypass walls. Combat exclusion and
the model size adjustment implement the explicit requested behavior.

## Quick in-game check

1. Refresh the browser after the server update, log in, and summon the pet.
2. Walk away: the pet should follow across ordinary slopes.
3. Open Setting, choose ON and the desired categories, then Confirm.
4. Drop an ordinary potion and equipment item nearby; check the pet Inventory.
5. Disable Other items and repeat the potion drop: it should stay on the ground.
6. Transfer a collected item to your own inventory, dismiss and resummon, and
   verify the remaining bag contents and settings survive.
