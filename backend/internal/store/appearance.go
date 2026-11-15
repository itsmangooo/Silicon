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
	Version int              `json:"version"`
	Theme   Theme            `json:"theme"`
	Themes  map[string]Theme `json:"themes,omitempty"`
	Presets []ThemePreset    `json:"presets"`
}

type ThemePreset struct {
	AppearanceBase
	Name  string `json:"name"`
	Theme Theme  `json:"theme"`
}

type Theme struct {
	Colors     map[string]string            `json:"colors,omitempty"`
	Background string                       `json:"background,omitempty"`
	Placement  string                       `json:"placement,omitempty"`
	Fit        string                       `json:"fit,omitempty"`
	Position   string                       `json:"position,omitempty"`
	Motion     string                       `json:"motion,omitempty"`
	Effects    map[string]int               `json:"effects,omitempty"`
	Surfaces   map[string]AppearanceSurface `json:"surfaces,omitempty"`
	Layout     *AppearanceLayout            `json:"layout,omitempty"`
}

type AppearanceSurface struct {
	Material        string            `json:"material,omitempty"`
	Color           string            `json:"color,omitempty"`
	BorderColor     string            `json:"borderColor,omitempty"`
	Border          string            `json:"border,omitempty"`
	Shadow          string            `json:"shadow,omitempty"`
	Opacity         *int              `json:"opacity,omitempty"`
	Blur            *int              `json:"blur,omitempty"`
	Radius          *int              `json:"radius,omitempty"`
	Density         string            `json:"density,omitempty"`
	TooltipMaterial string            `json:"tooltipMaterial,omitempty"`
	Colors          map[string]string `json:"colors,omitempty"`
}

type AppearanceLayout struct {
	Sidebar   string `json:"sidebar,omitempty"`
	Width     string `json:"width,omitempty"`
	Dashboard string `json:"dashboard,omitempty"`
}
