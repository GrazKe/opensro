/*
===========================================================================

petrecovery.go - optional stranded-pet recovery (port-only, not native)

Recovery uses the existing admitted companion spawn and cancels outstanding
work before moving the same pet identity. Its inventory and lease survive.

===========================================================================
*/
package action

import (
	"opensro.online/server/internal/domain"
	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/item/wire"
	"opensro.online/server/internal/game/world/simulation"
)

// These are local gameplay policies, not claimed native reconstruction.
const (
	petRecoveryDistance       = 1200.0
	petStuckDurationMs  int64 = 5000
	petProgressDistance       = 2.0
)

/*
================
petRecovery
================
*/
type petRecovery struct {
	started    bool
	progressAt int64
	anchor     simulation.Spawn
}

/*
================
needed

Measure actual displacement rather than attempted movement. Idle pets and
stationary attacks do not accrue a stuck timer.
================
*/
func (r *petRecovery) needed(pose, owner simulation.Spawn, pursuing bool, nowMs int64) bool {
	distance := simulation.WorldDistance2D(pose, owner)
	if distance > petRecoveryDistance {
		return true
	}
	if !pursuing {
		*r = petRecovery{}
		return false
	}
	if !r.started || nowMs < r.progressAt || simulation.WorldDistance2D(r.anchor, pose) >= petProgressDistance {
		*r = petRecovery{started: true, progressAt: nowMs, anchor: pose}
		return false
	}
	return nowMs-r.progressAt >= petStuckDurationMs
}

/*
================
petRecoveryStep
================
*/
type petRecoveryStep struct {
	key   petOwnerKey
	state *petSession
	pet   *enterworld.CharacterCOS
	owner simulation.Spawn
	nowMs int64
}

/*
================
recoverPet

The division lock owns recovery. Cancelled pickup releases its pending slot,
and retiring combat finalizes any upstream cast token before relocating.
================
*/
func (rt *Runtime) recoverPet(step petRecoveryStep) ([]simulation.Frame, bool) {
	state, pet, owner, nowMs := step.state, step.pet, step.owner, step.nowMs
	if !rt.PetPolicies.Recovery {
		state.recovery = petRecovery{}
		return nil, false
	}
	ref, found := rt.cosReference(pet)
	if !found || (ref.TidWord>>11 != domain.GrowthPetBand && ref.TidWord>>11 != domain.PickupPetBand) {
		return nil, false
	}
	world, _ := state.follower.Presentation()
	pose := state.follower.Position(nowMs)
	pursuing := world.MoveSegment.Valid() || state.pickup != nil || (state.combat == nil && simulation.WorldDistance2D(pose, owner) > simulation.PetFollowDistance)
	if state.combat != nil && state.combat.pursuing {
		pursuing = true
	}
	if !state.recovery.needed(pose, owner, pursuing, nowMs) {
		return nil, false
	}
	var frames []simulation.Frame
	if state.pickup != nil {
		result := finishPendingCosPickup(state, failureResult(wire.ErrCodeInvalidRequest))
		frames = append(frames, simFrames(result.Frames)...)
	}
	rt.cancelPetCombat(step.key, state, nowMs)
	state.recovery = petRecovery{}
	pose = rt.companionAdmissionSpawn(pet, owner)
	state.follower = simulation.NewPetFollower(pet.GID, pose)
	state.generation++
	correction := wire.Frame{Opcode: wire.OpObjectSourceCorrection, Payload: wire.ObjectSourceCorrection{Gid: pet.GID,
		Position: wire.Position{RegionID: pose.RegionID, X: float32(pose.X), Y: float32(pose.Y), Z: float32(pose.Z), Heading: pose.Angle}}.Encode()}
	state.public = append(state.public, correction)
	frames = append(frames, simFrames([]wire.Frame{correction})...)
	return frames, true
}
