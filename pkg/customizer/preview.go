// pkg/customizer/preview.go
package customizer

// GeneratePreviewHTML generates a preview HTML without redirect script
func GeneratePreviewHTML(c Customization) string {
	opts := GenerateOptions{
		Customization: c,
		RedirectURL:   "", // No redirect for preview
		Delay:         0,  // No delay for preview
	}
	return GenerateHTML(opts)
}
