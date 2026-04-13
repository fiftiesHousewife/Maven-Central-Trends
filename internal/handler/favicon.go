package handler

import "net/http"

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="32" height="32">
  <rect x="4" y="18" width="6" height="10" rx="1" fill="#3b82f6"/>
  <rect x="13" y="12" width="6" height="16" rx="1" fill="#10b981"/>
  <rect x="22" y="6" width="6" height="22" rx="1" fill="#3b82f6"/>
  <line x1="2" y1="29" x2="30" y2="29" stroke="#3b82f6" stroke-width="1.5" stroke-linecap="round"/>
</svg>`

// Favicon serves the SVG favicon for the analytics dashboard.
func Favicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write([]byte(faviconSVG))
}
