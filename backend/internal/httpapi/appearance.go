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

var themeHex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateAppearance(input store.Appearance) error {
	if !validAppearanceBase(input.AppearanceBase) {
		return errors.New("Select a supported style, color mode, and density.")
	}
	if input.Studio == nil {
		return nil
	}
	if input.Studio.Version != 2 || len(input.Studio.Presets) > 12 {
		return errors.New("Use Theme Studio version 2 with up to 12 saved presets.")
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
	return oneOf(input.Style, "soft", "glass", "playful", "obsidian", "industrial", "bento", "terminal") &&
		oneOf(input.Mode, "dark", "light", "system") && oneOf(input.Density, "comfortable", "compact")
}

func validateTheme(theme store.Theme, mode string) error {
	if !oneOf(theme.Background, "", "default", "solid", "gradient", "animated-gradient", "blobs", "image") ||
		!oneOf(theme.Placement, "", "application", "main", "sidebar", "dashboard") ||
		!oneOf(theme.Fit, "", "cover", "contain") || !oneOf(theme.Position, "", "center", "top", "bottom", "left", "right") ||
		!oneOf(theme.Motion, "", "system", "full", "reduced", "none") {
		return errors.New("Select supported background and motion options.")
	}
	for key, value := range theme.Colors {
		if !oneOf(key, "accent", "background", "secondary", "sidebar", "surface", "text", "muted", "border", "blob", "backgroundStart", "backgroundEnd") || !themeHex.MatchString(value) {
			return errors.New("Choose supported colors in #RRGGBB format.")
		}
	}
	for key, value := range theme.Effects {
		if !oneOf(key, "speed", "intensity", "blur", "colorIntensity", "blobSize", "blobOpacity", "imageOpacity", "overlay", "glassOpacity", "glassBlur", "glassReflection", "glassAnimation", "glassSpeed", "glassBlobOpacity") || value < 0 || value > 100 {
			return errors.New("Material controls must be between 0 and 100.")
		}
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
			a, b := themeLuminance(fg), themeLuminance(bg)
			if (max(a, b)+0.05)/(min(a, b)+0.05) < 4.5 {
				return false
			}
		}
	}
	return true
}

func themeLuminance(hex string) float64 {
	value, _ := strconv.ParseUint(hex[1:], 16, 32)
	linear := func(raw uint64) float64 {
		channel := float64(raw&255) / 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(value>>16) + 0.7152*linear(value>>8) + 0.0722*linear(value)
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
		config, format, err := image.DecodeConfig(bytes.NewReader(body))
		if err != nil || !oneOf(format, "png", "jpeg", "webp") || config.Width < 1 || config.Height < 1 || config.Width > 2560 || config.Height > 2560 || config.Width*config.Height > 2560*1440 {
			validation(w, "Upload a PNG, JPEG, or WebP up to 2560 pixels on either axis and 3.69 megapixels.")
			return
		}
		decoded, _, err := image.Decode(bytes.NewReader(body))
		if err != nil {
			validation(w, "This background image could not be decoded.")
			return
		}
		var clean bytes.Buffer
		if err := png.Encode(&clean, decoded); err != nil || clean.Len() > 4<<20 {
			validation(w, "The normalized background is too large. Choose a smaller image.")
			return
		}
		content = clean.Bytes()
	}
	if err := a.repo.SetAccountBackground(r.Context(), currentUser(r.Context()).ID, content); err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
