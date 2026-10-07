/*
===========================================================================

reversereturn_test.go - the guides' free reverse return and the gate scroll

===========================================================================
*/

package action

import (
	"bytes"
	"opensro.online/server/internal/domain"
	"opensro.online/server/internal/game/world/monster"
	"opensro.online/server/internal/testsupport/gamedatatest"
	"testing"
	"time"

	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/item/wire"
	"opensro.online/server/internal/game/world/simulation"
)

/*
================
reverseReturnFixture

The return fixture's character beside a selected teleport gate (gid 2001),
holding two reverse return scrolls in slot 23 (1000 ms cast).
================
*/
func reverseReturnFixture(t *testing.T) (*Runtime, *enterworld.Character, *fakeClock) {
	t.Helper()
	rt, c, clock, _ := returnFixture(t, 30000)
	ref := &enterworld.ItemRef{RefObjID: 3800, Codename: "ITEM_MALL_REVERSE_RETURN_SCROLL", Country: 3,
		TypeIDs: [4]int64{3, 3, 3, 3}, ReturnDestination: "RESURRECT",
		NativeFields: enterworld.NewNativeFields(map[string]float64{"canUse": 1, "itemParam1_29c": 1000, "itemParam2_2a0": 1})}
	rt.deps.(*enterworld.Deps).Items.(staticItemSource)[ref.Codename] = ref
	c.MissionInventory = append(c.MissionInventory, enterworld.InventoryRow{Slot: 23, RefObjID: ref.RefObjID,
		Codename: ref.Codename, TypeFlags: ref.TypeFlags(), StackCount: 2, VarianceBits: "0"})
	rt.NpcRoster = []simulation.NpcDef{{ObjectID: 2001, RefObjID: 2011, Codename: "NPC_CH_FERRY", TalkFlags: 2 | simulation.NpcTalkFlagTeleport,
		Services:      simulation.NpcServices(0).With(simulation.NpcServiceTeleport),
		AuthoredSpawn: true, Spawn: simulation.SeedWorldState(c).Spawn}}
	rt.NpcSpawn.Enabled = true
	rt.portals = &portalCatalog{sources: map[uint32]uint32{2011: 1}, destinations: map[uint32]portalDestination{1: {}}}
	rt.Selected.Set(testDivision, c.Name, 2001)
	return rt, c, clock
}

/*
================
reverseReturn
================
*/
func reverseReturn(rt *Runtime, c *enterworld.Character, choice uint8) OpResult {
	return rt.HandlePortal(testDivision, c, wire.NewWriter(6).U32(2001).U8(gateReverseReturn).U8(choice).Payload())
}

/*
================
TestGateOffersTheReverseReturnOnlyToAScrollHolder
================
*/
func TestGateOffersTheReverseReturnOnlyToAScrollHolder(t *testing.T) {
	rt, c, _ := reverseReturnFixture(t)
	gate := rt.NpcRoster[0]
	if rt.reverseReturnCapability(gate, c) != simulation.NpcTalkFlagReverseReturn {
		t.Fatal("a gate did not offer the reverse return to a scroll holder")
	}
	if rt.reverseReturnCapability(simulation.NpcDef{TalkFlags: 1}, c) != 0 {
		t.Fatal("a merchant offered the reverse return")
	}
	c.MissionInventory = c.MissionInventory[:len(c.MissionInventory)-1]
	if rt.reverseReturnCapability(gate, c) != 0 {
		t.Fatal("a gate offered the reverse return without a scroll")
	}
}

/*
================
guideFixture

The fixture's character, without scrolls, beside the selected beginner
guide NPC_EU_ADVICE3 (gid 2002) at max level 20.
================
*/
func guideFixture(t *testing.T) (*Runtime, *enterworld.Character) {
	t.Helper()
	rt, c, _ := reverseReturnFixture(t)
	c.MissionInventory = c.MissionInventory[:len(c.MissionInventory)-1]
	guide := simulation.NpcDef{ObjectID: 2002, RefObjID: 19519, Codename: "NPC_EU_ADVICE3",
		AuthoredSpawn: true, Spawn: simulation.SeedWorldState(c).Spawn}
	guide.Services = simulation.ResolveNpcServices(guide)
	guide.TalkFlags = simulation.ResolveNpcTalkFlags(guide)
	rt.NpcRoster = append(rt.NpcRoster, guide)
	rt.Selected.Set(testDivision, c.Name, guide.ObjectID)
	level := guideReturnMaxLevel
	c.MaxLevel = &level
	return rt, c
}

/*
================
guideReturn
================
*/
func guideReturn(rt *Runtime, c *enterworld.Character, choice uint8) OpResult {
	return rt.HandlePortal(testDivision, c, wire.NewWriter(6).U32(2002).U8(gateReverseReturn).U8(choice).Payload())
}

/*
================
TestGuideReverseReturnTravelsFreeToWhereItDied

4F2B50 case 5 moves a max level 20 character at no cost.
================
*/
func TestGuideReverseReturnTravelsFreeToWhereItDied(t *testing.T) {
	rt, c := guideFixture(t)
	if rt.NpcRoster[1].TalkFlags&simulation.NpcTalkFlagReverseReturn == 0 {
		t.Fatal("the guide's select word lacks the reverse return rows")
	}
	died := simulation.Spawn{RegionID: 25000, X: 812, Y: 30, Z: 1204, Angle: 0}
	c.World.LastDeathPoint = &domain.WorldPoint{WorldSpawn: *worldSpawnFromMission(died)}
	gold := goldOf(c)
	out := guideReturn(rt, c, reverseReturnLastDeath)
	if len(out.Frames) == 0 || out.Frames[0].Opcode != enterworld.OpcodeResetClient {
		t.Fatalf("the guide's return did not re-enter the world: %+v", out.Frames)
	}
	if got := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{}); got.RegionID != died.RegionID || got.X != died.X || got.Z != died.Z {
		t.Fatalf("arrived at %+v, not where the player died", got)
	}
	if goldOf(c) != gold || c.NativeTeleportMode != 0 {
		t.Fatal("the guide's return charged gold or started a scroll cast")
	}
}

/*
================
TestGuideReverseReturnRefusesInTheNativeOrder
================
*/
func TestGuideReverseReturnRefusesInTheNativeOrder(t *testing.T) {
	rt, c := guideFixture(t)
	before := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{})
	if out := guideReturn(rt, c, reverseReturnLastDeath); !bytes.Equal(out.Frames[0].Payload, portalFailure(errCodeGuideNoPoint).Frames[0].Payload) {
		t.Fatalf("missing death point answered %+v", out.Frames)
	}
	c.World.LastDeathPoint = &domain.WorldPoint{WorldSpawn: *worldSpawnFromMission(simulation.Spawn{RegionID: 25000, X: 812, Y: 30, Z: 1204})}
	level := guideReturnMaxLevel + 1
	c.MaxLevel = &level
	if out := guideReturn(rt, c, reverseReturnLastDeath); !bytes.Equal(out.Frames[0].Payload, portalFailure(errCodeGuideLevel).Frames[0].Payload) {
		t.Fatalf("max level 21 answered %+v", out.Frames)
	}
	rt.QuestTravelBlocks = func(*enterworld.Character) uint32 { return operationMaskQuestTravel }
	if out := guideReturn(rt, c, reverseReturnLastDeath); !bytes.Equal(out.Frames[0].Payload, portalFailure(errCodeGuideQuestBlock).Frames[0].Payload) {
		t.Fatalf("the quest travel block answered %+v", out.Frames)
	}
	rt.NpcRoster[1].Services = 0
	if out := guideReturn(rt, c, reverseReturnLastDeath); !bytes.Equal(out.Frames[0].Payload, portalFailure(errCodeTeleportService).Frames[0].Payload) {
		t.Fatalf("an NPC without a teleport service answered %+v", out.Frames)
	}
	if missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{}) != before {
		t.Fatal("a refused return moved the character")
	}
}

/*
================
TestReverseReturnWithoutADeathRefusesAndKeepsTheScroll

4A00C0: no recorded death answers 0x1886 (notice 390) and spends nothing.
================
*/
func TestReverseReturnWithoutADeathRefusesAndKeepsTheScroll(t *testing.T) {
	rt, c, _ := reverseReturnFixture(t)
	out := reverseReturn(rt, c, reverseReturnLastDeath)
	if len(out.Frames) != 1 || !bytes.Equal(out.Frames[0].Payload, wire.EncodeItemUseError(errCodeNoDeathPoint)) {
		t.Fatalf("missing death point answered %+v", out.Frames)
	}
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
		t.Fatal("a refused reverse return spent the scroll or started a cast")
	}
}

/*
================
TestReverseReturnTakesThePlayerToWhereItDied
================
*/
func TestReverseReturnTakesThePlayerToWhereItDied(t *testing.T) {
	rt, c, clock := reverseReturnFixture(t)
	died := simulation.Spawn{RegionID: 25000, X: 812, Y: 30, Z: 1204, Angle: 0}
	c.World.LastDeathPoint = &domain.WorldPoint{WorldSpawn: *worldSpawnFromMission(died)}
	out := reverseReturn(rt, c, reverseReturnLastDeath)
	assertOpcodes(t, out.Frames, 0x3122, wire.OpItemUseResponse, wire.OpItemUseVisual)
	if out.Frames[1].Payload[1] != 23 || c.MissionInventory[len(c.MissionInventory)-1].StackCount != 1 || c.NativeTeleportMode != 1 {
		t.Fatalf("the reverse return did not spend one scroll and start the cast: %+v", out.Frames)
	}
	var sent []wire.Frame
	rt.PushCharacterFrames = func(_, _ string, f []wire.Frame) { sent = append(sent, f...) }
	clock.Advance(time.Second)
	rt.TickHook()(clock.NowMs())
	if len(sent) == 0 || sent[0].Opcode != enterworld.OpcodeResetClient {
		t.Fatalf("missing native reentry: %+v", sent)
	}
	if got := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{}); got.RegionID != died.RegionID || got.X != died.X || got.Z != died.Z {
		t.Fatalf("arrived at %+v, not where the player died", got)
	}
}

/*
================
TestReturnScrollAndDeathRecordTheReverseReturnPoints
================
*/
func TestReturnScrollAndDeathRecordTheReverseReturnPoints(t *testing.T) {
	rt, c, clock := reverseReturnFixture(t)
	at := rt.liveSpawn(simulation.WorldKey(testDivision, c.Name), c, clock.NowMs())
	useReturn(rt, c)
	if c.World.LastRecallPoint == nil || missionSpawnFromWorld(&c.World.LastRecallPoint.WorldSpawn, simulation.Spawn{}) != at {
		t.Fatalf("the return scroll recorded %+v, want %+v", c.World.LastRecallPoint, at)
	}
	rt.settlePlayerDeathInDoor(testDivision, c, deathKiller{}, clock.NowMs())
	if c.World.LastDeathPoint == nil || missionSpawnFromWorld(&c.World.LastDeathPoint.WorldSpawn, simulation.Spawn{}).RegionID != at.RegionID {
		t.Fatalf("death recorded %+v", c.World.LastDeathPoint)
	}
}

/*
================
TestRecordedPointsKeepTheirWorld

4E0250 / 4E0330 record a point only in a type-0 world and keep its
GameWorldID: the field is the absent default, a fortress names itself, and
an instance dungeon (type 1) records nothing.
================
*/
func TestRecordedPointsKeepTheirWorld(t *testing.T) {
	at := simulation.Spawn{RegionID: 17221, X: 812, Z: 677}
	c := &enterworld.Character{}
	if point, ok := recordedPoint(c, at); !ok || point.World != 0 {
		t.Fatalf("field point %+v %v", point, ok)
	}
	for _, tc := range []struct {
		packed uint32
		world  uint16
		ok     bool
	}{{0x10002, 2, true}, {0x1000a, 0, false}} {
		packed := tc.packed
		c.World = &domain.CharacterWorld{PackedInstance: &packed}
		point, ok := recordedPoint(c, at)
		if ok != tc.ok || ok && point.World != tc.world {
			t.Fatalf("world %08x: %+v %v", tc.packed, point, ok)
		}
	}
}

/*
================
TestInventoryReverseScrollRecordedChoices
================
*/
func TestInventoryReverseScrollRecordedChoices(t *testing.T) {
	for _, choice := range []uint8{reverseReturnLastRecall, reverseReturnLastDeath} {
		t.Run(string(rune('0'+choice)), func(t *testing.T) {
			rt, c, clock := reverseReturnFixture(t)
			destination := simulation.Spawn{RegionID: 25000, X: 812, Y: 30, Z: 1204}
			point := &domain.WorldPoint{WorldSpawn: *worldSpawnFromMission(destination)}
			if choice == reverseReturnLastRecall {
				c.World.LastRecallPoint = point
			} else {
				c.World.LastDeathPoint = point
			}
			rt.Selected.Set(testDivision, c.Name, 0)
			row := c.MissionInventory[len(c.MissionInventory)-1]
			payload := wire.NewWriter(4).U8(uint8(row.Slot)).U16(row.TypeFlags).U8(choice).Payload()
			out := rt.HandleItemUse(testDivision, c, payload)
			assertOpcodes(t, out.Frames, 0x3122, wire.OpItemUseResponse, wire.OpItemUseVisual)
			if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 1 || c.NativeTeleportMode != 1 {
				t.Fatal("bag reverse did not consume exactly one and start casting")
			}
			rt.HandleItemUse(testDivision, c, payload)
			if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 1 {
				t.Fatal("duplicate use consumed another scroll")
			}
			clock.Advance(time.Second)
			rt.TickHook()(clock.NowMs())
			got := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{})
			if got.RegionID != destination.RegionID || got.X != destination.X || got.Z != destination.Z {
				t.Fatalf("reverse arrived at %+v", got)
			}
		})
	}
}

/*
================
TestInventoryReverseScrollRefusalsPreserveStack
================
*/
func TestInventoryReverseScrollRefusalsPreserveStack(t *testing.T) {
	for _, tail := range [][]byte{nil, {reverseReturnLastRecall}, {reverseReturnLastDeath}, {99}, {2, 1}, {4, 0, 0}, {4, 1, 0, 0}} {
		rt, c, _ := reverseReturnFixture(t)
		row := c.MissionInventory[len(c.MissionInventory)-1]
		payload := append(wire.NewWriter(3).U8(uint8(row.Slot)).U16(row.TypeFlags).Payload(), tail...)
		out := rt.HandleItemUse(testDivision, c, payload)
		if len(out.Frames) != 1 || out.Frames[0].Opcode != wire.OpItemUseResponse || out.Frames[0].Payload[0] == 1 {
			t.Fatalf("invalid destination accepted: %+v", out)
		}
		if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 || c.NativeTeleportMode != 0 {
			t.Fatal("refusal spent a scroll")
		}
	}
}

/*
================
TestReverseScrollMapUsesRecallSpawnAndServerIds
================
*/
func TestReverseScrollMapUsesRecallSpawnAndServerIds(t *testing.T) {
	rt, c, clock := reverseReturnFixture(t)
	spawn := simulation.Spawn{RegionID: 25000, X: 900, Y: 30, Z: 1000}
	rt.portals.destinations[1] = portalDestination{recall: true, code: "GATE_CH", world: 1, spawn: spawn}
	t.Setenv(envReverseMap, "0")
	if len(rt.ReverseScrollPoints()) != 0 {
		t.Fatal("native default exposed map extension")
	}
	row := c.MissionInventory[len(c.MissionInventory)-1]
	payload := wire.NewWriter(6).U8(uint8(row.Slot)).U16(row.TypeFlags).U8(reverseScrollMapChoice).U16(1).Payload()
	rt.HandleItemUse(testDivision, c, payload)
	if c.MissionInventory[len(c.MissionInventory)-1].StackCount != 2 {
		t.Fatal("disabled map consumed scroll")
	}
	t.Setenv(envReverseMap, "1")
	points := rt.ReverseScrollPoints()
	if len(points) != 1 || points[0].ID != 1 || points[0].RegionID != spawn.RegionID || points[0].X != spawn.X || points[0].Y != spawn.Y || points[0].Z != spawn.Z {
		t.Fatalf("town point diverges from recall: %+v", points)
	}
	out := rt.HandleItemUse(testDivision, c, payload)
	assertOpcodes(t, out.Frames, 0x3122, wire.OpItemUseResponse, wire.OpItemUseVisual)
	clock.Advance(time.Second)
	rt.TickHook()(clock.NowMs())
	got := missionSpawnFromWorld(c.World.Spawn, simulation.Spawn{})
	if got.RegionID != spawn.RegionID || got.X != spawn.X || got.Z != spawn.Z {
		t.Fatalf("map arrived at %+v", got)
	}
}

/*
================
TestReverseScrollRetailCatalogueIncludesTownsAndUniqueAnchors
================
*/
func TestReverseScrollRetailCatalogueIncludesTownsAndUniqueAnchors(t *testing.T) {
	dir := gamedatatest.TextdataDir(t)
	rt, c, _ := reverseReturnFixture(t)
	catalog, err := loadPortalCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	rt.portals = catalog
	rt.Monsters = simulation.NewMonsterState(monster.LoadTemplate(dir))
	t.Setenv(envReverseMap, "1")
	points := rt.ReverseScrollPoints()
	if len(points) < 6 {
		t.Fatalf("retail map catalogue missing unique anchors: %+v", points)
	}
	for _, code := range []string{"GATE_CH", "GATE_WC", "GATE_KT", "GATE_EU", "GATE_CA"} {
		found := false
		for _, gate := range catalog.destinations {
			if gate.code != code {
				continue
			}
			for _, point := range points {
				if point.RegionID == gate.spawn.RegionID && point.X == gate.spawn.X && point.Y == gate.spawn.Y && point.Z == gate.spawn.Z {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("town recall missing: %s", code)
		}
	}
	for _, point := range points {
		if point.ID == 0 || point.RegionID >= 0x8000 || point.X < 0 || point.X >= 1920 || point.Z < 0 || point.Z >= 1920 {
			t.Fatalf("invalid retail point %+v", point)
		}
	}
	t.Logf("%d map destinations for %s", len(points), c.Name)
}
