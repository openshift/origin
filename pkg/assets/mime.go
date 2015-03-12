package assets

import "mime"

// RegisterMimeTypes adds mime type registrations for the file types the assets server will serve.
// Registering here so we work without /etc/mime.types installed
func RegisterMimeTypes() {
	// Text types
	registerIfNeeded(".css", "text/css; charset=utf-8")
	registerIfNeeded(".html", "text/html; charset=utf-8")
	registerIfNeeded(".txt", "text/plain; charset=utf-8")

	// Image types
	registerIfNeeded(".ico", "image/vnd.microsoft.icon")
	registerIfNeeded(".png", "image/png")
	registerIfNeeded(".svg", "image/svg+xml")

	// JavaScript types
	registerIfNeeded(".js", "application/javascript; charset=utf-8")
	registerIfNeeded(".json", "application/json; charset=utf-8")

	// Font types
	// http://www.iana.org/assignments/media-types/application/vnd.ms-fontobject
	registerIfNeeded(".eot", "application/vnd.ms-fontobject")
	// http://www.w3.org/TR/WOFF/#appendix-b
	registerIfNeeded(".woff", "application/font-woff")
	// http://www.iana.org/assignments/media-types/application/font-sfnt
	registerIfNeeded(".ttf", "application/font-sfnt")
	registerIfNeeded(".otf", "application/font-sfnt")

	// Flash
	registerIfNeeded(".swf", "application/x-shockwave-flash")
}

func registerIfNeeded(extension, mimeType string) {
	if mime.TypeByExtension(extension) == "" {
		mime.AddExtensionType(extension, mimeType)
	}
}
