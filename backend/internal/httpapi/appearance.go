package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/itsmangooo/Silicon/backend/internal/store"
	_ "golang.org/x/image/webp"
)

var themeHex = regexp.MustCompile(`^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$`)

func validateAppearance(input store.Appearance) error {
	if !validAppearanceBase(input.AppearanceBase) {
		return errors.New("Select a supported style, color mode, and density.")
	}
	if input.Studio == nil {
		return nil
	}
	if (input.Studio.Version != 2 && input.Studio.Version != 3) || len(input.Studio.Presets) > 12 {
		return errors.New("Use a supported Theme Studio version with up to 12 saved presets.")
	}
	if err := validateTheme(input.Studio.Theme, input.Mode); err != nil {
		return err
	}
	names := make(map[string]bool, len(input.Studio.Presets))
	for _, preset := range input.Studio.Presets {
		name := strings.ToLower(strings.TrimSpace(preset.Name))
		if name == "" || len(preset.Name) > 64 || names[name] || strings.ContainsFunc(preset.Name, unicode.IsControl) {
			return errors.New("Give each preset a unique name up to 64 bytes.")
		}
		names[name] = true
		if !validAppearanceBase(preset.AppearanceBase) {
			return errors.New("A saved preset uses an unsupported style, mode, or density.")
		}
		if err := validateTheme(preset.Theme, preset.Mode); err != nil {
			return fmt.Errorf("Preset %q: %w", preset.Name, err)
		}
	}
	return nil
}

func validAppearanceBase(input store.AppearanceBase) bool {
	return oneOf(input.Style, "glass", "playful", "obsidian", "soft", "neumorphism", "clay", "bento", "flat") &&
		oneOf(input.Mode, "dark", "light", "system") && oneOf(input.Density, "comfortable", "compact")
}

func validateTheme(theme store.Theme, mode string) error {
	if !oneOf(theme.Background, "", "default", "solid", "gradient", "radial", "mesh", "animated-gradient", "blobs", "image") ||
		!oneOf(theme.Placement, "", "application", "main", "sidebar", "dashboard") ||
		!oneOf(theme.Fit, "", "cover", "contain") || !oneOf(theme.Position, "", "center", "top", "bottom", "left", "right") ||
		!oneOf(theme.Motion, "", "system", "full", "reduced", "none") {
		return errors.New("Select supported background and motion options.")
	}
	for key, value := range theme.Colors {
		if !oneOf(key, "accent", "background", "secondary", "sidebar", "surface", "text", "muted", "border", "header", "panel", "input", "button", "dialog", "success", "warning", "danger", "info", "blob", "glassTint", "backgroundStart", "backgroundEnd") || !themeHex.MatchString(value) {
			return errors.New("Choose supported colors in #RRGGBB or #RRGGBBAA format.")
		}
	}
	for key, value := range theme.Effects {
		if !oneOf(key, "speed", "intensity", "blur", "colorIntensity", "blobSize", "blobOpacity", "blobCount", "imageOpacity", "imageSize", "imageRepeat", "imageBrightness", "imageSaturation", "overlay", "glassOpacity", "glassBlur", "glassReflection", "glassAnimation", "glassSpeed", "glassBlobOpacity", "glassSaturation") || value < 0 || value > 100 {
			return errors.New("Material controls must be between 0 and 100.")
		}
	}
	if err := validateThemeSurfaces(theme); err != nil {
		return err
	}
	if mode != "light" && !themeContrast(theme.Colors, false) || mode != "dark" && !themeContrast(theme.Colors, true) {
		return errors.New("Increase contrast between text, muted text, links, and backgrounds, or select a matching color mode.")
	}
	return nil
}

func themeContrast(colors map[string]string, light bool) bool {
	base := map[string]string{"background": "#101114", "secondary": "#202227", "sidebar": "#131417", "surface": "#191B1F", "text": "#E7E8EA", "muted": "#A7AAB1", "accent": "#A1AEC7"}
	if light {
		base = map[string]string{"background": "#F0F1F3", "secondary": "#E8E9ED", "sidebar": "#F8F8F9", "surface": "#FFFFFF", "text": "#202228", "muted": "#555B66", "accent": "#475771"}
	}
	for _, foreground := range []string{"text", "muted", "accent"} {
		for _, background := range []string{"background", "secondary", "sidebar", "surface"} {
			if colors[foreground] == "" && colors[background] == "" {
				continue
			}
			fg, bg := base[foreground], base[background]
			if colors[foreground] != "" {
				fg = colors[foreground]
			}
			if colors[background] != "" {
				bg = colors[background]
			}
			backdrop := compositeThemeColor(bg, hexRGBA(base[background]))
			a, b := relativeLuminance(compositeThemeColor(fg, backdrop)), relativeLuminance(backdrop)
			if (max(a, b)+0.05)/(min(a, b)+0.05) < 4.5 {
				return false
			}
		}
	}
	return true
}

func hexRGBA(hex string) [4]float64 {
	if len(hex) == 7 {
		hex += "ff"
	}
	value, _ := strconv.ParseUint(hex[1:], 16, 32)
	return [4]float64{float64((value>>24)&255) / 255, float64((value>>16)&255) / 255, float64((value>>8)&255) / 255, float64(value&255) / 255}
}

func compositeThemeColor(foreground string, background [4]float64) [4]float64 {
	rgba := hexRGBA(foreground)
	for i := range 3 {
		background[i] = rgba[i]*rgba[3] + background[i]*(1-rgba[3])
	}
	return background
}

func relativeLuminance(rgb [4]float64) float64 {
	linear := func(channel float64) float64 {
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(rgb[0]) + 0.7152*linear(rgb[1]) + 0.0722*linear(rgb[2])
}

func validateThemeSurfaces(theme store.Theme) error {
	if layout := theme.Layout; layout != nil && (!oneOf(layout.Sidebar, "", "standard", "compact", "floating", "collapsed") ||
		!oneOf(layout.Width, "", "comfortable", "wide", "full") || !oneOf(layout.Dashboard, "", "standard", "modular")) {
		return errors.New("Select supported layout options.")
	}
	for name, surface := range theme.Surfaces {
		if !oneOf(name, "sidebar", "header", "card", "panel", "widget", "input", "button", "dialog", "table", "chart", "widget-activity", "widget-runtime", "widget-servers") ||
			!oneOf(surface.Material, "", "solid", "transparent", "glass", "elevated", "raised", "inset", "clay", "flat") ||
			!oneOf(surface.Border, "", "none", "thin") || !oneOf(surface.Shadow, "", "none", "low", "medium", "high") ||
			!boundedThemeValue(surface.Opacity, 100) || !boundedThemeValue(surface.Blur, 32) || !boundedThemeValue(surface.Radius, 32) {
			return errors.New("Select supported surface options.")
		}
		if surface.Density != "" && (name != "table" || !oneOf(surface.Density, "compact", "comfortable")) ||
			surface.TooltipMaterial != "" && (name != "chart" || !oneOf(surface.TooltipMaterial, "solid", "glass")) {
			return errors.New("Select supported table or chart options.")
		}
		if (surface.Color != "" && !themeHex.MatchString(surface.Color)) || (surface.BorderColor != "" && !themeHex.MatchString(surface.BorderColor)) {
			return errors.New("Enter surface colors in #RRGGBB or #RRGGBBAA format.")
		}
		for key, value := range surface.Colors {
			if !themeHex.MatchString(value) || !validSurfaceColor(name, key) {
				return errors.New("Select a supported surface color.")
			}
		}
	}
	return nil
}

func boundedThemeValue(value *int, maximum int) bool {
	return value == nil || (*value >= 0 && *value <= maximum)
}

func validSurfaceColor(surface, color string) bool {
	switch surface {
	case "input":
		return color == "focus"
	case "button":
		return oneOf(color, "primary", "secondary", "hover", "pressed")
	case "table":
		return oneOf(color, "heading", "hover", "selection", "text")
	case "chart":
		return oneOf(color, "grid", "series", "seriesSecondary", "tooltip", "legend")
	}
	return false
}

func (a *API) accountBackground(w http.ResponseWriter, r *http.Request) {
	content, err := a.repo.AccountBackground(r.Context(), currentUser(r.Context()).ID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(content)
}

func (a *API) updateAccountBackground(w http.ResponseWriter, r *http.Request) {
	var content []byte
	if r.Method == http.MethodPut {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "background_too_large", "Upload an image up to 1 MiB.")
			return
		}
		content, err = normalizeBackground(body)
		if err != nil {
			validation(w, err.Error())
			return
		}
	}
	if err := a.repo.SetAccountBackground(r.Context(), currentUser(r.Context()).ID, content); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeBackground(body []byte) ([]byte, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || !oneOf(format, "png", "jpeg", "webp") || config.Width < 1 || config.Height < 1 || config.Width > 2560 || config.Height > 2560 || config.Width*config.Height > 2560*1440 {
		return nil, errors.New("Upload a PNG, JPEG, or WebP up to 2560 pixels on either axis and 3.69 megapixels.")
	}
	decoded, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("This background image could not be decoded.")
	}
	var clean bytes.Buffer
	if err := png.Encode(&clean, decoded); err != nil || clean.Len() > 4<<20 {
		return nil, errors.New("The normalized background is too large. Choose a smaller image.")
	}
	return clean.Bytes(), nil
}
