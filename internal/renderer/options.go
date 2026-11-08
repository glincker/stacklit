package renderer

// timestampsEnabled controls whether WriteJSON stamps generated_at. Leaving it
// off makes the output a pure function of the repo, so it never churns in git.
var timestampsEnabled = true

// SetTimestamps turns the generated_at stamp on or off for later writes.
func SetTimestamps(on bool) { timestampsEnabled = on }

// remoteIcons lets the HTML map request language logos from thesvg.org. When
// off, the page carries no remote URL and shows letter monograms instead.
var remoteIcons = true

// SetMapIcons turns remote icon requests in the HTML map on or off.
func SetMapIcons(on bool) { remoteIcons = on }
