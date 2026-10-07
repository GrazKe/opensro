/*
===========================================================================

speed.go - shared grab and growth pet pacing for movement and spawn packets

===========================================================================
*/
package companion

import (
	"os"
	"strings"
)

// EnvPetPacing is port-only, not native; unset retains authored movement.
const EnvPetPacing = "SRO_PET_PACING"

// EnvPetRecovery is port-only, not native; unset disables custom relocation.
const EnvPetRecovery = "SRO_PET_RECOVERY"

/*
================
PolicyEnabled
================
*/
func PolicyEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "on":
		return true
	default:
		return false
	}
}

const petRunSpeedScale = 0.8

/*
================
RunSpeed

Local gameplay policy: grab and growth pets run twenty percent slower.
Vehicle and captured quest actor speeds retain their authored values.
================
*/
func RunSpeed(band uint16, speed float32) float32 {
	if PolicyEnabled(EnvPetPacing) && (band == 3 || band == 4) {
		return speed * petRunSpeedScale
	}
	return speed
}
