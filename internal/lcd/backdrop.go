package lcd

import "image/color"

// BackgroundColor is the colour a non-transparent render starts from. A
// transparent HUD plus a plain picture of this colour looks the same as the
// opaque render.
func (r *Renderer) BackgroundColor() color.RGBA { return r.bg }
