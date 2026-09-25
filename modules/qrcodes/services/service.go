package services

import (
	"bytes"
	"database/sql"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/botginx/botginx/modules/qrcodes/models"
	"github.com/jmoiron/sqlx"
	"github.com/skip2/go-qrcode"
)

type QRCodeService struct {
	db *sqlx.DB
}

func NewQRCodeService(db *sqlx.DB) *QRCodeService {
	return &QRCodeService{db: db}
}

func (s *QRCodeService) List(userID string) ([]models.QRCode, error) {
	var codes []models.QRCode
	err := s.db.Select(&codes, `
		SELECT * FROM qr_codes
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *QRCodeService) ListPaginated(userID string, page, limit int) ([]models.QRCode, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	err := s.db.Get(&total, `SELECT COUNT(*) FROM qr_codes WHERE user_id = $1`, userID)
	if err != nil {
		return nil, 0, err
	}

	var codes []models.QRCode
	err = s.db.Select(&codes, `
		SELECT * FROM qr_codes
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

func (s *QRCodeService) Get(id string) (*models.QRCode, error) {
	var code models.QRCode
	err := s.db.Get(&code, `SELECT * FROM qr_codes WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func (s *QRCodeService) GetByUser(id, userID string) (*models.QRCode, error) {
	var code models.QRCode
	err := s.db.Get(&code, `SELECT * FROM qr_codes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func (s *QRCodeService) Create(userID string, req models.CreateQRCodeRequest) (*models.QRCode, error) {
	if req.FGColor == "" {
		req.FGColor = "#000000"
	}
	if req.BGColor == "" {
		req.BGColor = "#FFFFFF"
	}

	var redirectLinkID sql.NullString
	if req.RedirectLinkID != "" {
		redirectLinkID = sql.NullString{String: req.RedirectLinkID, Valid: true}
	}

	var code models.QRCode
	err := s.db.Get(&code, `
		INSERT INTO qr_codes (user_id, title, url, redirect_link_id, fg_color, bg_color)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING *
	`, userID, req.Title, req.URL, redirectLinkID, req.FGColor, req.BGColor)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func (s *QRCodeService) Update(id, userID string, req models.UpdateQRCodeRequest) (*models.QRCode, error) {
	var code models.QRCode
	err := s.db.Get(&code, `
		UPDATE qr_codes
		SET title = $1, url = $2, fg_color = $3, bg_color = $4, updated_at = NOW()
		WHERE id = $5 AND user_id = $6
		RETURNING *
	`, req.Title, req.URL, req.FGColor, req.BGColor, id, userID)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func (s *QRCodeService) Delete(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM qr_codes WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func (s *QRCodeService) Count(userID string) (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM qr_codes WHERE user_id = $1`, userID)
	return count, err
}

// GeneratePNG generates a QR code as PNG bytes
func GeneratePNG(url string, size int, fgColor, bgColor string) ([]byte, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return nil, err
	}

	qr.ForegroundColor = parseHexColor(fgColor)
	qr.BackgroundColor = parseHexColor(bgColor)

	var buf bytes.Buffer
	err = qr.Write(size, &buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GenerateSVG generates a QR code as SVG string
func GenerateSVG(url string, size int, fgColor, bgColor string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}

	bitmap := qr.Bitmap()
	moduleCount := len(bitmap)
	if moduleCount == 0 {
		return "", fmt.Errorf("empty QR code")
	}

	moduleSize := size / moduleCount
	if moduleSize < 1 {
		moduleSize = 1
	}
	actualSize := moduleSize * moduleCount

	var svg strings.Builder
	svg.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`, actualSize, actualSize, actualSize, actualSize))
	svg.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, bgColor))

	for y, row := range bitmap {
		for x, cell := range row {
			if cell {
				svg.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
					x*moduleSize, y*moduleSize, moduleSize, moduleSize, fgColor))
			}
		}
	}

	svg.WriteString(`</svg>`)
	return svg.String(), nil
}

func parseHexColor(hex string) color.Color {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.Black
	}

	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}
