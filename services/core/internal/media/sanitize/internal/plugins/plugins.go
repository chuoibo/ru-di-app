package plugins

import "mobile/services/core/internal/media/sanitize/internal/pil"

// ByName returns the 36 open plugins this package reproduces, keyed by
// Pillow's format name.
func ByName() map[string]pil.Plugin {
	list := []pil.Plugin{
		plugin("AVIF", acceptAVIF, openAVIF),
		plugin("BLP", acceptBLP, openBLP),
		stub("BUFR", 4, acceptBUFR),
		plugin("CUR", acceptCUR, openCUR),
		plugin("PCX", acceptPCX, openPCX),
		plugin("DCX", acceptDCX, openDCX),
		plugin("DDS", acceptDDS, openDDS),
		plugin("EPS", acceptEPS, openEPS),
		plugin("FITS", acceptFITS, openFITS),
		plugin("FLI", acceptFLI, openFLI),
		plugin("FTEX", acceptFTEX, openFTEX),
		plugin("GBR", acceptGBR, openGBR),
		stub("GRIB", 8, acceptGRIB),
		stub("HDF5", 8, acceptHDF5),
		plugin("JPEG2000", acceptJPEG2000, openJPEG2000),
		plugin("ICNS", acceptICNS, openICNS),
		plugin("ICO", acceptICO, openICO),
		plugin("IM", nil, openIM),
		plugin("IMT", nil, openIMT),
		plugin("IPTC", nil, openIPTC),
		plugin("MCIDAS", acceptMCIDAS, openMCIDAS),
		plugin("MPEG", func(p []byte) bool { return hasPrefix(p, "\x00\x00\x01\xb3") }, openMPEG),
		plugin("TIFF", acceptTIFF, openTIFF),
		plugin("MSP", acceptMSP, openMSP),
		plugin("PCD", nil, openPCD),
		plugin("PIXAR", acceptPIXAR, openPIXAR),
		plugin("PSD", acceptPSD, openPSD),
		plugin("QOI", acceptQOI, openQOI),
		plugin("SGI", acceptSGI, openSGI),
		plugin("SPIDER", nil, openSPIDER),
		plugin("SUN", acceptSUN, openSUN),
		plugin("TGA", nil, openTGA),
		plugin("WMF", acceptWMF, openWMF),
		plugin("XBM", acceptXBM, openXBM),
		plugin("XPM", acceptXPM, openXPM),
		plugin("XVTHUMB", acceptXVTHUMB, openXVTHUMB),
	}
	out := make(map[string]pil.Plugin, len(list))
	for _, p := range list {
		out[p.Name] = p
	}
	return out
}
