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
	c, refs := persistentSummonFixture()
	rt, _ := newTestRuntime(c, refs)
	rt.PetPolicies = companion.Policies{Pacing: true, Recovery: true}
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
	c, refs := persistentSummonFixture()
	rt, _ := newTestRuntime(c, refs)
	rt.PetPolicies = companion.Policies{Pacing: true, Recovery: true}
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

Pacing slows only the run a grab or growth pet moves with; the native
parameter the follow and battle rules compare (548A30) keeps its value.
================
*/
func TestPetRunSpeedPolicyLeavesVehiclesAndWalkingAlone(t *testing.T) {
	rt := &Runtime{PetPolicies: companion.Policies{Pacing: true}}
	for _, band := range []uint16{1, 2, 3, 4, 6} {
		ref := &enterworld.CharacterRef{TidWord: 0x1c6 | band<<11, RunSpeed: 100, WalkSpeed: 20}
		want := float32(100)
		if band == 3 || band == 4 {
			want = 80
		}
		walk, run := rt.cosMovementSpeeds(ref, nil, nil)
		if run != want || walk != 20 {
			t.Fatalf("band %d moves at %v/%v, want 20/%v", band, walk, run, want)
		}
		if got := cosParameter(ref, nil, nil, movementRunParameter); got != 100 {
			t.Fatalf("band %d native run parameter paced to %v", band, got)
		}
	}
}

/*
================
TestPacedFollowSpeedMatchesNativeAtFullFactor

A paced pet already at its native full speed installs nothing: the battle
reset (548A30) compares the native parameter, not the paced one, so it
does not resend the speed every tick.
================
*/
func TestPacedFollowSpeedMatchesNativeAtFullFactor(t *testing.T) {
	c, refs := persistentSummonFixture()
	rt, _ := newTestRuntime(c, refs)
	rt.PetPolicies.Pacing = true
	ref := refs.characters["ATTACK"]
	pet := &enterworld.CharacterCOS{GID: 42, Codename: "ATTACK", RefObjID: ref.RefObjID, CurrentHP: 100, Summoned: true}
	run := cosParameter(ref, pet, nil, movementRunParameter)
	if run < ref.RunSpeed {
		t.Fatalf("native run %v below authored %v: the battle reset would fire", run, ref.RunSpeed)
	}
	state := &petSession{follower: simulation.NewPetFollower(pet.GID, simulation.Spawn{RegionID: 0x62aa}), character: c}
	step := petCombatStep{key: petOwnerKey{division: testDivision, name: strings.ToLower(c.Name), gid: pet.GID}, state: state,
		snapshot: c, pet: pet, ref: ref, run: run, nowMs: 1000}
	if frames := rt.setCompanionFollowSpeed(step, 100); len(frames) != 0 {
		t.Fatal("full factor resent the follow speed", frames)
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
	if policies := companion.PoliciesFromEnv(); policies != (companion.Policies{}) {
		t.Fatalf("unset environment enabled %+v", policies)
	}
	t.Setenv(companion.EnvPetRecovery, "on")
	t.Setenv(companion.EnvPetPacing, "1")
	if policies := companion.PoliciesFromEnv(); policies != (companion.Policies{Pacing: true, Recovery: true}) {
		t.Fatalf("set environment read as %+v", policies)
	}
	c, refs := persistentSummonFixture()
	rt, _ := newTestRuntime(c, refs)
	if rt.PetPolicies != (companion.Policies{}) || rt.PetPolicies.RunSpeed(4, 100) != 100 {
		t.Fatal("a new runtime is not native")
	}
	ref := refs.characters["PICKUP"]
	pet := &enterworld.CharacterCOS{GID: 42, Codename: "PICKUP", RefObjID: ref.RefObjID, CurrentHP: 100}
	state := &petSession{follower: simulation.NewPetFollower(pet.GID, simulation.Spawn{RegionID: 0x62aa}), character: c}
	_, recovered := rt.recoverPet(petRecoveryStep{state: state, pet: pet, owner: simulation.Spawn{RegionID: 0x62aa, X: 2000}, nowMs: 1000})
	if recovered {
		t.Fatal("default enabled custom recovery")
	}
}
