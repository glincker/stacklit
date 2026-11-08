package renderer

// timestampsEnabled controls whether WriteJSON stamps generated_at. Leaving it
// off makes the output a pure function of the repo, so it never churns in git.
var timestampsEnabled = true

// SetTimestamps turns the generated_at stamp on or off for later writes.
func SetTimestamps(on bool) { timestampsEnabled = on }
