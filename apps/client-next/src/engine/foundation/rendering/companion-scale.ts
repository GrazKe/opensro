/*
===========================================================================

companion-scale.ts - the requested grab-pet model size adjustment

Only pickup companions shrink; attack pets, mounts and captured monsters
retain their existing scale. The same factor feeds geometry and effects.

===========================================================================
*/
const PICKUP_PET_SCALE = 0.75;
const COS_TYPE_MASK = 0x7fe;
const COS_TYPE = 0x1c6;
const PICKUP_PET_BAND = 4;

/*
================
companionModelScale
================
*/
export function companionModelScale( tidWord: number, enabled = false ): number {
	// Port-only, not native: this optional presentation adjustment is off by default.
	return enabled && (tidWord & COS_TYPE_MASK) === COS_TYPE && (tidWord >>> 11) === PICKUP_PET_BAND ?
		PICKUP_PET_SCALE :
		1;
}
