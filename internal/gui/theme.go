package gui

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/style"
)

// The house type, embedded so the window looks the same on every machine: Fira Sans for
// words, Fira Code for ids, dates and keys. OFL.txt sits beside them.
var (
	//go:embed fonts/FiraSans-Regular.ttf
	firaRegular []byte
	//go:embed fonts/FiraSans-SemiBold.ttf
	firaSemiBold []byte
	//go:embed fonts/FiraSans-Italic.ttf
	firaItalic []byte
	//go:embed fonts/FiraSans-SemiBoldItalic.ttf
	firaSemiBoldItalic []byte
	//go:embed fonts/FiraCode-Regular.ttf
	firaCode []byte
)

var fonts = struct{ regular, bold, italic, boldItalic, mono fyne.Resource }{
	fyne.NewStaticResource("FiraSans-Regular.ttf", firaRegular),
	fyne.NewStaticResource("FiraSans-SemiBold.ttf", firaSemiBold),
	fyne.NewStaticResource("FiraSans-Italic.ttf", firaItalic),
	fyne.NewStaticResource("FiraSans-SemiBoldItalic.ttf", firaSemiBoldItalic),
	fyne.NewStaticResource("FiraCode-Regular.ttf", firaCode),
}

// sizeProse is the help page's reading size: a touch above the UI's 14, well under a heading.
const sizeProse fyne.ThemeSizeName = "prose"

// variant is the one the whole window wears. The theme decides it once at start (from
// the desktop, or the test) rather than per lookup: the header and the window must agree
// on which half of every adaptive colour they draw, and the test settings report a
// variant that is "not a preference".
var variant = theme.VariantLight

func dark() bool { return variant == theme.VariantDark }

// rgba is a palette entry in the window's variant.
func rgba(c lipgloss.AdaptiveColor) color.RGBA { return style.RGBA(c, dark()) }

// palette is style's colours worn as a Fyne theme. Everything it does not name falls
// through to the default, so a widget never shows a colour the palette does not own.
type palette struct{ base fyne.Theme }

func newTheme(v fyne.ThemeVariant) fyne.Theme {
	variant = v
	return &palette{base: theme.DefaultTheme()}
}

func (p *palette) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground, theme.ColorNameInputBackground:
		return rgba(style.Paper)
	case theme.ColorNameForeground:
		return rgba(style.Ink)
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return rgba(style.Subtle)
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return rgba(style.Accent)
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError,
		theme.ColorNameForegroundOnWarning, theme.ColorNameForegroundOnSuccess:
		return rgba(style.Paper)
	case theme.ColorNameSelection, theme.ColorNamePressed:
		return rgba(style.AccentSoft)
	case theme.ColorNameHover, theme.ColorNameButton, theme.ColorNameDisabledButton,
		theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground, theme.ColorNameHeaderBackground:
		return rgba(style.Card)
	case theme.ColorNameInputBorder, theme.ColorNameSeparator, theme.ColorNameScrollBar:
		return rgba(style.Line)
	case theme.ColorNameScrollBarBackground:
		return color.Transparent // the thin bar alone; a track is a stripe the paper does not need
	case theme.ColorNameShadow:
		// Straight alpha, not RGBA's premultiplied: a translucent colour written the other
		// way composites as a purple glow.
		c := rgba(style.Line)
		return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x60}
	case theme.ColorNameError:
		return rgba(style.High)
	case theme.ColorNameWarning:
		return rgba(style.Review)
	case theme.ColorNameSuccess:
		return rgba(style.Low)
	}
	return p.base.Color(name, variant)
}

func (p *palette) Font(s fyne.TextStyle) fyne.Resource {
	switch {
	case s.Monospace:
		return fonts.mono
	case s.Bold && s.Italic:
		return fonts.boldItalic
	case s.Bold:
		return fonts.bold
	case s.Italic:
		return fonts.italic
	}
	return fonts.regular
}

func (p *palette) Icon(n fyne.ThemeIconName) fyne.Resource { return p.base.Icon(n) }

func (p *palette) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 14
	case theme.SizeNameCaptionText:
		return 12
	case sizeProse:
		return 15
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 3
	}
	return p.base.Size(n)
}
