/*
===========================================================================

petrecovery_test.go - recovery deadlines, progress and shared pet speed policy

===========================================================================
*/
package action

import (
	"strings"
	"testing"

	"opensro.online/server/internal/game/companion"
	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/item/wire"
	"opensro.online/server/internal/game/world/simulation"
)

/*
================
TestPetRecoveryProgressAndIdle
================
*/
func TestPetRecoveryProgressAndIdle(t *testing.T) {
	t.Setenv(companion.EnvPetRecovery, "1")
	t.Setenv(companion.EnvPetPacing, "1")
	owner := simulation.Spawn{RegionID: 0x62aa, X: 400}
	pose := simulation.Spawn{RegionID: owner.RegionID}
	var recovery petRecovery
	if recovery.needed(pose, owner, true, 1000) || recovery.needed(pose, owner, true, 5999) {
		t.Fatal("recovered before stuck deadline")
	}
	pose.X = 3
	if recovery.needed(pose, owner, true, 6000) || recovery.needed(pose, owner, true, 10999) {
		t.Fatal("movement did not reset stuck deadline")
	}
	if !recovery.needed(pose, owner, true, 11000) {
		t.Fatal("stranded pet did not recover")
	}
	if recovery.needed(pose, owner, false, 20000) || recovery.needed(pose, owner, true, 20001) {
		t.Fatal("idle time counted as stuck")
	}
	owner.X = petRecoveryDistance + pose.X + 1
	if !recovery.needed(pose, owner, false, 20002) {
		t.Fatal("distant pet did not recover immediately")
	}
}

/*
================
TestPetTickRecoversBlockedPickupAndAcknowledgesCancellation
================
*/
func TestPetTickRecoversBlockedPickupAndAcknowledgesCancellation(t *testing.T) {
	t.Setenv(companion.EnvPetRecovery, "1")
	t.Setenv(companion.EnvPetPacing, "1")
	c, refs := persistentSummonFixture()
	rt, _ := newTestRuntime(c, refs)
	rt.BindPetSession(testDivision, c, 101)
	useSummonerFixture(t, rt, c, 24, refs.staticItemSource["SUMMON_PICKUP"])
	pet := c.Companions()[0]
	key := petOwnerKey{division: testDivision, name: strings.ToLower(c.Name), gid: pet.GID}
	state := rt.petSessions[key]
	owner := rt.liveSpawn(simulation.WorldKey(testDivision, c.Name), c, 1000)
	stranded := owner
	stranded.X += petRecoveryDistance + 100
	state.follower = simulation.NewPetFollower(pet.GID, stranded)
	state.pickup = &wire.ItemMoveRequest{CosGID: pet.GID, GroundGID: 123}
	state.pickupCommand = true
	state.pickupDeadline = 10000
	frames := rt.advancePet(key, 1000)
	var receipts []wire.Frame
	for _, frame := range frames {
		receipts = append(receipts, wire.Frame{Opcode: frame.Opcode, Payload: frame.Payload})
	}
	assertCosPickupAck(t, receipts, pet.GID, 123, false)
	if state.pickup != nil || simulation.WorldDistance2D(state.follower.Position(1000), owner) > float64(companionSpawnRadius) {
		t.Fatal("tick failed to retire pickup and recover pet")
	}
}

/*
================
TestPetRecoveryKeepsIdentityAndCancelsCombat
================
*/
func TestPetRecoveryKeepsIdentityAndCancelsCombat(t *testing.T) {
	t.Setenv(companion.EnvPetRecovery, "1")
	t.Setenv(companion.EnvPetPacing, "1")
	c, refs := persistentSummonFixture()
	rt, _ := newTestRuntime(c, refs)
	rt.CompanionRoll = func() (uint32, error) { return 0, nil }
	owner := simulation.Spawn{RegionID: 0x62aa, X: 1500}
	for _, name := range []string{"ATTACK", "PICKUP"} {
		ref := refs.characters[name]
		pet := &enterworld.CharacterCOS{GID: 42, Codename: name, RefObjID: ref.RefObjID, CurrentHP: 100}
		state := &petSession{follower: simulation.NewPetFollower(pet.GID, simulation.Spawn{RegionID: owner.RegionID}), combat: &petCombatIntent{}}
		frames, recovered := rt.recoverPet(petRecoveryStep{key: petOwnerKey{division: testDivision, gid: pet.GID}, state: state, pet: pet, owner: owner, nowMs: 1000})
		if !recovered || len(frames) != 1 || len(state.public) != 1 || state.combat != nil || state.follower.GID() != pet.GID || pet.CurrentHP != 100 {
			t.Fatalf("%s recovery changed identity or retained combat: %+v", name, state)
		}
		if simulation.WorldDistance2D(state.follower.Position(1000), owner) > float64(companionSpawnRadius) {
			t.Fatal("recovery did not place pet beside owner")
		}
	}
}

/*
================
TestPetRunSpeedPolicyLeavesVehiclesAndWalkingAlone
================
*/
func TestPetRunSpeedPolicyLeavesVehiclesAndWalkingAlone(t *testing.T) {
	t.Setenv(companion.EnvPetRecovery, "1")
	t.Setenv(companion.EnvPetPacing, "1")
	for _, band := range []uint16{1, 2, 3, 4, 6} {
		ref := &enterworld.CharacterRef{TidWord: 0x1c6 | band<<11, RunSpeed: 100, WalkSpeed: 20}
		want := float32(100)
		if band == 3 || band == 4 {
			want = 80
		}
		if got := cosParameter(ref, nil, nil, movementRunParameter); got != want {
			t.Fatalf("band %d run %v, want %v", band, got, want)
		}
		if got := cosParameter(ref, nil, nil, movementWalkParameter); got != 20 {
			t.Fatalf("band %d walking changed: %v", band, got)
		}
	}
}

/*
================
TestPetCustomPoliciesDefaultOff
================
*/
func TestPetCustomPoliciesDefaultOff(t *testing.T) {
	t.Setenv(companion.EnvPetRecovery, "")
	t.Setenv(companion.EnvPetPacing, "")
	if companion.RunSpeed(4, 100) != 100 {
		t.Fatal("default pet speed changed")
	}
	c, refs := persistentSummonFixture()
	rt, _ := newTestRuntime(c, refs)
	ref := refs.characters["PICKUP"]
	pet := &enterworld.CharacterCOS{GID: 42, Codename: "PICKUP", RefObjID: ref.RefObjID, CurrentHP: 100}
	state := &petSession{follower: simulation.NewPetFollower(pet.GID, simulation.Spawn{RegionID: 0x62aa}), character: c}
	_, recovered := rt.recoverPet(petRecoveryStep{state: state, pet: pet, owner: simulation.Spawn{RegionID: 0x62aa, X: 2000}, nowMs: 1000})
	if recovered {
		t.Fatal("default enabled custom recovery")
	}
}
