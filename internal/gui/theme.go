package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/cheese-cracker/tray/internal/style"
)

// palette is style's colours worn as a Fyne theme. Everything it does not name falls
// through to the default, so a widget never shows a colour the palette does not own.
type palette struct{ base fyne.Theme }

func newTheme() fyne.Theme { return &palette{base: theme.DefaultTheme()} }

func (p *palette) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == theme.VariantDark
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return style.RGBA(style.Accent, dark)
	case theme.ColorNameSelection:
		c := style.RGBA(style.Accent, dark)
		c.A = 0x40
		return c
	case theme.ColorNameError:
		return style.RGBA(style.High, dark)
	case theme.ColorNameWarning:
		return style.RGBA(style.Review, dark)
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return style.RGBA(style.Subtle, dark)
	}
	return p.base.Color(name, variant)
}

func (p *palette) Font(s fyne.TextStyle) fyne.Resource     { return p.base.Font(s) }
func (p *palette) Icon(n fyne.ThemeIconName) fyne.Resource { return p.base.Icon(n) }
func (p *palette) Size(n fyne.ThemeSizeName) float32       { return p.base.Size(n) }

// dark says which half of an adaptive colour the running app wants.
func dark() bool {
	return fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark
}
