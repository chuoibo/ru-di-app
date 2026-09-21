package sanitize

import (
	"mobile/services/core/internal/media/sanitize/internal/bmpdec"
	"mobile/services/core/internal/media/sanitize/internal/gifdec"
	"mobile/services/core/internal/media/sanitize/internal/imageopen"
	"mobile/services/core/internal/media/sanitize/internal/jpegdec"
	"mobile/services/core/internal/media/sanitize/internal/pil"
	"mobile/services/core/internal/media/sanitize/internal/plugins"
	"mobile/services/core/internal/media/sanitize/internal/pngdec"
	"mobile/services/core/internal/media/sanitize/internal/ppm"
	"mobile/services/core/internal/media/sanitize/internal/webpdec"
)

// registry is Image.OPEN: every plugin named in imageopen.Order.
var registry = buildRegistry()

func buildRegistry() map[string]pil.Plugin {
	all := plugins.ByName()
	for _, plugin := range []pil.Plugin{
		bmpdec.Plugin, bmpdec.DIBPlugin, gifdec.Plugin, jpegdec.Plugin,
		ppm.Plugin, pngdec.Plugin, webpdec.Plugin,
	} {
		all[plugin.Name] = plugin
	}
	for _, name := range imageopen.Order {
		if _, ok := all[name]; !ok {
			panic("sanitize: no plugin for " + name)
		}
	}
	return all
}
