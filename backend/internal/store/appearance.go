package store

type Appearance struct {
	AppearanceBase
	Studio *ThemeStudio `json:"studio,omitempty"`
}

type AppearanceBase struct {
	Style   string `json:"style"`
	Mode    string `json:"mode"`
	Density string `json:"density"`
}

type ThemeStudio struct {
	Version int           `json:"version"`
	Theme   Theme         `json:"theme"`
	Presets []ThemePreset `json:"presets"`
}

type ThemePreset struct {
	AppearanceBase
	Name  string `json:"name"`
	Theme Theme  `json:"theme"`
}

type Theme struct {
	Colors     map[string]string `json:"colors,omitempty"`
	Background string            `json:"background,omitempty"`
	Placement  string            `json:"placement,omitempty"`
	Fit        string            `json:"fit,omitempty"`
	Position   string            `json:"position,omitempty"`
	Motion     string            `json:"motion,omitempty"`
	Quality    string            `json:"quality,omitempty"`
	Effects    map[string]int    `json:"effects,omitempty"`
}
