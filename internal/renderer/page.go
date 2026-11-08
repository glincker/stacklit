package renderer

import (
	"strings"

	"github.com/glincker/stacklit/assets"
)

const iconHosts = "https://thesvg.org https://cdn.jsdelivr.net"

// renderPage fills the embedded template. With remote icons off, the slug map
// is replaced by an empty one and the CSP allows no remote image host.
func renderPage(dataJSON, graphJSON string) string {
	langIcons, hosts := assets.LangIconsJS, iconHosts
	if !remoteIcons {
		langIcons, hosts = "var LANG_ICONS = {}; var ICON_BASES = []; var MARK_SLUG = '';", ""
	}
	html := strings.Replace(assets.TemplateHTML, "{{ICON_HOSTS}}", hosts, 1)
	html = strings.Replace(html, "{{STACKLIT_DATA}}", dataJSON, 1)
	html = strings.Replace(html, "{{STACKLIT_GRAPH}}", graphJSON, 1)
	html = strings.Replace(html, "{{SYMBOLS_JS}}", assets.SymbolsJS, 1)
	return strings.Replace(html, "{{LANG_ICONS_JS}}", langIcons, 1)
}
