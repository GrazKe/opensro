/*
===========================================================================

reverse_scroll.go - inventory reverse scroll choices and optional map points

The v1.150 item 3/3/3/3 and textuisystem entries define the recorded-point
prompt. INFERENCE: bag use sends the selected point as a byte after the
normal item identity and uses the same return cast as the gate scroll.
Map selection is a later-version extension (port-only, not native), off
unless SRO_REVERSE_MAP=1. Its inferred points are existing town recall
positions and outdoor unique nest anchors, not unrestricted coordinate warps.

===========================================================================
*/
package action

import (
	"encoding/binary"
	"math"
	"opensro.online/server/internal/domain"
	"opensro.online/server/internal/game/enterworld"
	"opensro.online/server/internal/game/item/wire"
	"opensro.online/server/internal/game/world/instance"
	"os"
	"sort"
	"strings"
)

const reverseScrollMapChoice uint8 = 4
const envReverseMap = "SRO_REVERSE_MAP"
const maxReverseScrollPoints = 4096

/*
================
ReverseScrollPoints

Stable ordering fixes ids across the public catalogue and use validation.
Town entries use precisely the same recall spawn as appointed rebirth.
================
*/
func (rt *Runtime) ReverseScrollPoints() []enterworld.ReverseScrollPoint {
	if os.Getenv(envReverseMap) != "1" {
		return nil
	}
	var points []enterworld.ReverseScrollPoint
	if rt.portals != nil {
		for _, gate := range rt.portals.destinations {
			if !gate.recall || gate.fortressGate || gate.spawn.RegionID == 0 || gate.spawn.RegionID&0x8000 != 0 || portalWorld(gate) != instance.ID(domain.DefaultWorldInstance) {
				continue
			}
			name := strings.TrimPrefix(gate.code, "GATE_")
			if town, ok := map[string]string{"CH": "Jangan", "WC": "Donwhang", "KT": "Hotan", "EU": "Constantinople", "CA": "Samarkand"}[name]; ok {
				name = town
			}
			points = append(points, enterworld.ReverseScrollPoint{Name: name, RegionID: gate.spawn.RegionID, X: gate.spawn.X, Y: gate.spawn.Y, Z: gate.spawn.Z})
		}
	}
	if rt.Monsters != nil {
		for _, anchor := range rt.Monsters.UniqueReturnAnchors() {
			ref, ok := rt.Monsters.Reference(anchor.RefObjID)
			if !ok {
				continue
			}
			name := ref.Name
			if name == "" {
				name = ref.Codename
			}
			points = append(points, enterworld.ReverseScrollPoint{Name: name, RegionID: anchor.RegionID, X: anchor.X, Y: anchor.Y, Z: anchor.Z})
		}
	}
	sort.Slice(points, func(i, j int) bool {
		a, b := points[i], points[j]
		if a.RegionID != b.RegionID {
			return a.RegionID < b.RegionID
		}
		if a.X != b.X {
			return a.X < b.X
		}
		if a.Z != b.Z {
			return a.Z < b.Z
		}
		return a.Name < b.Name
	})
	var result []enterworld.ReverseScrollPoint
	previous := struct {
		region uint16
		x, z   float64
	}{}
	for _, point := range points {
		if point.X < 0 || point.X >= 1920 || point.Z < 0 || point.Z >= 1920 || math.IsNaN(point.X) || math.IsNaN(point.Y) || math.IsNaN(point.Z) || math.IsInf(point.X, 0) || math.IsInf(point.Y, 0) || math.IsInf(point.Z, 0) {
			continue
		}
		key := struct {
			region uint16
			x, z   float64
		}{point.RegionID, point.X, point.Z}
		if key == previous {
			continue
		}
		previous = key
		if len(result) >= maxReverseScrollPoints {
			break
		}
		point.ID = uint16(len(result) + 1)
		result = append(result, point)
	}
	return result
}

/*
================
reverseScrollUse

Called inside the normal item-use transaction and division lock. No state
changes occur before identity, destination and shared return admission pass.
================
*/
type reverseScrollUse struct {
	division  string
	character *enterworld.Character
	row       int
	request   wire.ItemUseRequest
	ref       *enterworld.ItemRef
	tail      []byte
}

/*
================
useReverseScroll
================
*/
func (rt *Runtime) useReverseScroll(use reverseScrollUse, result *OpResult) bool {
	division, c, row, request, ref, tail := use.division, use.character, use.row, use.request, use.ref, use.tail
	if len(tail) != 1 && len(tail) != 3 {
		return false
	}
	choice := tail[0]
	var destination travelPoint
	if choice == reverseReturnLastRecall || choice == reverseReturnLastDeath {
		if len(tail) != 1 {
			return false
		}
		var refusal uint8
		destination, refusal = reverseReturnPoint(c, choice)
		if refusal != 0 {
			*result = itemUseFailure(refusal)
			return false
		}
	} else if choice == reverseScrollMapChoice && len(tail) == 3 {
		id := binary.LittleEndian.Uint16(tail[1:])
		found := false
		for _, point := range rt.ReverseScrollPoints() {
			if point.ID != id {
				continue
			}
			destination = travelPoint{world: instance.ID(domain.DefaultWorldInstance)}
			destination.spawn.RegionID = point.RegionID
			destination.spawn.X, destination.spawn.Y, destination.spawn.Z = point.X, point.Y, point.Z
			found = true
			break
		}
		if !found {
			return false
		}
	} else {
		return false
	}
	duration, ok := returnScrollDuration(ref)
	if !ok || !rt.returnScrollAdmission(division, c, result) {
		return false
	}
	return rt.startReturnCast(returnCast{division: division, character: c, row: row, slot: request.Slot, typeWord: request.TypeWord, duration: duration, destination: &destination, now: rt.Now().UnixMilli()}, result)
}
