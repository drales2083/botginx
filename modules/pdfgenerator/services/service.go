package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/jung-kurt/gofpdf"
)

type PDFService struct {
	backgroundsFS fs.FS
}

func NewPDFService(backgroundsFS fs.FS) *PDFService {
	return &PDFService{
		backgroundsFS: backgroundsFS,
	}
}

// PDFRequest contains all settings for PDF generation
type PDFRequest struct {
	// Background
	ImageDataURL   string `json:"imageDataUrl"`   // Base64 data URL from upload
	PresetImage    string `json:"presetImage"`    // Preset image filename
	BlurLevel      int    `json:"blurLevel"`      // 0-20 pixels

	// Button
	ButtonText   string `json:"buttonText"`
	ButtonColor  string `json:"buttonColor"`  // hex color
	TextColor    string `json:"textColor"`    // hex color
	BorderRadius int    `json:"borderRadius"` // px
	PaddingX     int    `json:"paddingX"`     // px
	PaddingY     int    `json:"paddingY"`     // px
	FontSize     int    `json:"fontSize"`     // px
	Shadow       bool   `json:"shadow"`

	// Link
	LinkURL string `json:"linkUrl"`

	// PDF settings
	PageSize    string `json:"pageSize"`    // a4, letter
	Orientation string `json:"orientation"` // portrait, landscape
	Margins     int    `json:"margins"`     // mm
}

// GeneratePreview creates a PNG preview image with button overlay
func (s *PDFService) GeneratePreview(req PDFRequest) ([]byte, error) {
	// Load and process background image
	bgImg, err := s.loadBackgroundImage(req)
	if err != nil {
		return nil, fmt.Errorf("load background: %w", err)
	}

	// Apply blur
	if req.BlurLevel > 0 {
		bgImg = imaging.Blur(bgImg, float64(req.BlurLevel))
	}

	// Resize for preview (A4 ratio ~1:1.414)
	previewWidth := 400
	previewHeight := int(float64(previewWidth) * 1.414)
	if req.Orientation == "landscape" {
		previewWidth, previewHeight = previewHeight, previewWidth
	}

	bgImg = imaging.Fill(bgImg, previewWidth, previewHeight, imaging.Center, imaging.Lanczos)

	// Note: For simplicity, the button is drawn via CSS overlay in the frontend preview
	// The actual PDF renders the button natively

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, bgImg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GeneratePDF creates the final PDF with clickable button
func (s *PDFService) GeneratePDF(req PDFRequest) ([]byte, error) {
	log.Printf("[pdfgenerator] GeneratePDF called: presetImage=%q, imageDataURL_len=%d, blurLevel=%d",
		req.PresetImage, len(req.ImageDataURL), req.BlurLevel)
	// Determine page size
	var pageWidth, pageHeight float64
	switch req.PageSize {
	case "letter":
		pageWidth, pageHeight = 215.9, 279.4 // mm
	default: // a4
		pageWidth, pageHeight = 210, 297 // mm
	}

	orientation := "P"
	if req.Orientation == "landscape" {
		orientation = "L"
		pageWidth, pageHeight = pageHeight, pageWidth
	}

	// Create PDF - gofpdf expects "A4" or "Letter" (capitalized)
	pageSize := "A4"
	if req.PageSize == "letter" {
		pageSize = "Letter"
	}
	pdf := gofpdf.New(orientation, "mm", pageSize, "")
	pdf.SetMargins(float64(req.Margins), float64(req.Margins), float64(req.Margins))
	pdf.AddPage()

	// Load and process background
	bgImg, err := s.loadBackgroundImage(req)
	if err != nil {
		return nil, fmt.Errorf("load background: %w", err)
	}

	// Apply blur
	if req.BlurLevel > 0 {
		bgImg = imaging.Blur(bgImg, float64(req.BlurLevel))
	}

	// Resize to page dimensions (convert mm to pixels at 150 DPI)
	dpi := 150.0
	imgWidth := int(pageWidth * dpi / 25.4)
	imgHeight := int(pageHeight * dpi / 25.4)
	bgImg = imaging.Fill(bgImg, imgWidth, imgHeight, imaging.Center, imaging.Lanczos)

	// Encode background as JPEG and register with PDF
	var imgBuf bytes.Buffer
	if err := jpeg.Encode(&imgBuf, bgImg, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}

	pdf.RegisterImageOptionsReader("bg", gofpdf.ImageOptions{ImageType: "jpeg"}, bytes.NewReader(imgBuf.Bytes()))
	pdf.ImageOptions("bg", 0, 0, pageWidth, pageHeight, false, gofpdf.ImageOptions{}, 0, "")

	// Calculate button position (centered)
	buttonText := req.ButtonText
	if buttonText == "" {
		buttonText = "Click Here"
	}

	// Apply defaults
	fontSize := req.FontSize
	if fontSize <= 0 {
		fontSize = 18
	}
	paddingXpx := req.PaddingX
	if paddingXpx <= 0 {
		paddingXpx = 40
	}
	paddingYpx := req.PaddingY
	if paddingYpx <= 0 {
		paddingYpx = 16
	}
	borderRadius := req.BorderRadius
	if borderRadius < 0 {
		borderRadius = 8
	}

	// Set up font for button text
	fontPt := float64(fontSize) * 0.75
	pdf.SetFont("Helvetica", "B", fontPt)

	// Calculate button dimensions
	textWidth := pdf.GetStringWidth(buttonText)
	paddingX := float64(paddingXpx) * 0.264583 // px to mm
	paddingY := float64(paddingYpx) * 0.264583
	buttonWidth := textWidth + (paddingX * 2)
	buttonHeight := fontPt*0.352778 + (paddingY * 2) // font pt to mm + padding

	// Center position
	buttonX := (pageWidth - buttonWidth) / 2
	buttonY := (pageHeight - buttonHeight) / 2

	// Parse button color
	r, g, b := hexToRGB(req.ButtonColor)

	// Draw button shadow if enabled
	radius := float64(borderRadius) * 0.264583
	if req.Shadow {
		pdf.SetFillColor(0, 0, 0)
		pdf.SetAlpha(0.3, "Normal")
		pdf.RoundedRect(buttonX+2, buttonY+2, buttonWidth, buttonHeight, radius, "1234", "F")
		pdf.SetAlpha(1.0, "Normal")
	}

	// Draw button background
	pdf.SetFillColor(r, g, b)
	pdf.RoundedRect(buttonX, buttonY, buttonWidth, buttonHeight, radius, "1234", "F")

	// Draw button text
	tr, tg, tb := hexToRGB(req.TextColor)
	pdf.SetTextColor(tr, tg, tb)
	textX := buttonX + paddingX
	textY := buttonY + paddingY + (fontPt * 0.352778 * 0.8) // baseline adjustment
	pdf.Text(textX, textY, buttonText)

	// Add clickable link annotation over the button
	if req.LinkURL != "" {
		pdf.LinkString(buttonX, buttonY, buttonWidth, buttonHeight, req.LinkURL)
	}

	// Check for PDF errors before output
	if pdf.Err() {
		return nil, fmt.Errorf("pdf error: %s", pdf.Error())
	}

	// Output PDF
	var pdfBuf bytes.Buffer
	if err := pdf.Output(&pdfBuf); err != nil {
		return nil, fmt.Errorf("pdf output error: %w", err)
	}

	return pdfBuf.Bytes(), nil
}

// loadBackgroundImage loads from data URL or preset
func (s *PDFService) loadBackgroundImage(req PDFRequest) (image.Image, error) {
	log.Printf("[pdfgenerator] loadBackgroundImage: presetImage=%q, hasDataURL=%v", req.PresetImage, req.ImageDataURL != "")
	var imgReader io.Reader

	if req.ImageDataURL != "" {
		// Decode base64 data URL
		parts := strings.SplitN(req.ImageDataURL, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URL format")
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("decode base64: %w", err)
		}
		imgReader = bytes.NewReader(decoded)
	} else if req.PresetImage != "" {
		// Load from embedded filesystem
		f, err := s.backgroundsFS.Open(req.PresetImage)
		if err != nil {
			return nil, fmt.Errorf("open preset %s: %w", req.PresetImage, err)
		}
		defer f.Close()
		imgReader = f
	} else {
		return nil, fmt.Errorf("no image provided")
	}

	// Decode image
	img, format, err := image.Decode(imgReader)
	if err != nil {
		log.Printf("[pdfgenerator] image decode failed: %v", err)
		return nil, fmt.Errorf("decode image: %w", err)
	}
	log.Printf("[pdfgenerator] image decoded: %dx%d, format=%s", img.Bounds().Dx(), img.Bounds().Dy(), format)

	return img, nil
}

// hexToRGB converts hex color to RGB values
func hexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 220, 53, 69 // default red
	}

	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}
