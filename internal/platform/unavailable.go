//go:build (!windows && !darwin) || !cgo

package platform

func Run(Options, func(Event)) error { return ErrUnavailable }
func Present([]Command)              {}
func Wake()                          {}
func Quit()                          {}
func RenderedFrames() uint64         { return 0 }
func MeasureText(text string, size float32) (float32, float32) {
	return float32(len([]rune(text))) * size * 0.6, size * 1.4
}
func WindowAction(int)           {}
func RendererStats() RenderStats { return RenderStats{Backend: "unavailable"} }
func MeasureTextWithFont(text string, size float32, _ string) (float32, float32) {
	return MeasureText(text, size)
}
