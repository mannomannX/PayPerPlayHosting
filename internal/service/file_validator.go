package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/png" // PNG decoder
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/payperplay/hosting/internal/models"
)

// FileValidator validates uploaded files
type FileValidator interface {
	Validate(file multipart.File, header *multipart.FileHeader) error
	GetMaxSizeMB() int64
	GetAllowedExtensions() []string
	GetDescription() string
	GetFileType() models.FileType
}

// ===== Resource Pack Validator =====

type ResourcePackValidator struct{}

func NewResourcePackValidator() FileValidator {
	return &ResourcePackValidator{}
}

func (v *ResourcePackValidator) Validate(file multipart.File, header *multipart.FileHeader) error {
	// Check extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		return fmt.Errorf("resource pack must be a .zip file")
	}

	// Check size
	if header.Size > v.GetMaxSizeMB()*1024*1024 {
		return fmt.Errorf("resource pack too large (max %d MB)", v.GetMaxSizeMB())
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Reset file pointer for later use
	file.Seek(0, 0)

	// Validate ZIP structure
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("invalid ZIP file: %w", err)
	}

	// Check for pack.mcmeta (required for resource packs)
	hasMcMeta := false
	for _, f := range zipReader.File {
		if f.Name == "pack.mcmeta" || strings.HasSuffix(f.Name, "/pack.mcmeta") {
			hasMcMeta = true
			break
		}
	}

	if !hasMcMeta {
		return fmt.Errorf("resource pack must contain pack.mcmeta file")
	}

	return nil
}

func (v *ResourcePackValidator) GetMaxSizeMB() int64 {
	return 100 // 100 MB max for resource packs
}

func (v *ResourcePackValidator) GetAllowedExtensions() []string {
	return []string{".zip"}
}

func (v *ResourcePackValidator) GetDescription() string {
	return "Minecraft Resource Pack (.zip, max 100 MB, must contain pack.mcmeta)"
}

func (v *ResourcePackValidator) GetFileType() models.FileType {
	return models.FileTypeResourcePack
}

// ===== Data Pack Validator =====

type DataPackValidator struct{}

func NewDataPackValidator() FileValidator {
	return &DataPackValidator{}
}

func (v *DataPackValidator) Validate(file multipart.File, header *multipart.FileHeader) error {
	// Check extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		return fmt.Errorf("data pack must be a .zip file")
	}

	// Check size
	if header.Size > v.GetMaxSizeMB()*1024*1024 {
		return fmt.Errorf("data pack too large (max %d MB)", v.GetMaxSizeMB())
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Validate ZIP structure
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("invalid ZIP file: %w", err)
	}

	// Check for pack.mcmeta (required for data packs)
	hasMcMeta := false
	hasData := false
	for _, f := range zipReader.File {
		if f.Name == "pack.mcmeta" || strings.HasSuffix(f.Name, "/pack.mcmeta") {
			hasMcMeta = true
		}
		if strings.Contains(f.Name, "/data/") {
			hasData = true
		}
	}

	if !hasMcMeta {
		return fmt.Errorf("data pack must contain pack.mcmeta file")
	}

	if !hasData {
		return fmt.Errorf("data pack must contain /data/ folder")
	}

	return nil
}

func (v *DataPackValidator) GetMaxSizeMB() int64 {
	return 50 // 50 MB max for data packs
}

func (v *DataPackValidator) GetAllowedExtensions() []string {
	return []string{".zip"}
}

func (v *DataPackValidator) GetDescription() string {
	return "Minecraft Data Pack (.zip, max 50 MB, must contain pack.mcmeta and /data/)"
}

func (v *DataPackValidator) GetFileType() models.FileType {
	return models.FileTypeDataPack
}

// ===== Server Icon Validator =====

type ServerIconValidator struct{}

func NewServerIconValidator() FileValidator {
	return &ServerIconValidator{}
}

func (v *ServerIconValidator) Validate(file multipart.File, header *multipart.FileHeader) error {
	// Check extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".png") {
		return fmt.Errorf("server icon must be a .png file")
	}

	// Check size
	if header.Size > v.GetMaxSizeMB()*1024*1024 {
		return fmt.Errorf("server icon too large (max %d MB)", v.GetMaxSizeMB())
	}

	// Decode image to check dimensions
	img, format, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("invalid PNG image: %w", err)
	}

	// Reset file pointer
	file.Seek(0, 0)

	if format != "png" {
		return fmt.Errorf("image must be PNG format")
	}

	// Check dimensions (must be exactly 64x64)
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width != 64 || height != 64 {
		return fmt.Errorf("server icon must be exactly 64x64 pixels (got %dx%d)", width, height)
	}

	return nil
}

func (v *ServerIconValidator) GetMaxSizeMB() int64 {
	return 1 // 1 MB max for icons
}

func (v *ServerIconValidator) GetAllowedExtensions() []string {
	return []string{".png"}
}

func (v *ServerIconValidator) GetDescription() string {
	return "Server Icon (64x64 PNG, max 1 MB)"
}

func (v *ServerIconValidator) GetFileType() models.FileType {
	return models.FileTypeServerIcon
}

// ===== World Gen Validator =====

type WorldGenValidator struct{}

func NewWorldGenValidator() FileValidator {
	return &WorldGenValidator{}
}

func (v *WorldGenValidator) Validate(file multipart.File, header *multipart.FileHeader) error {
	// Check extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".json") {
		return fmt.Errorf("world generation file must be a .json file")
	}

	// Check size
	if header.Size > v.GetMaxSizeMB()*1024*1024 {
		return fmt.Errorf("world generation file too large (max %d MB)", v.GetMaxSizeMB())
	}

	// TODO: Validate JSON schema for Minecraft world gen format
	// This would require parsing the JSON and checking against MC schema

	return nil
}

func (v *WorldGenValidator) GetMaxSizeMB() int64 {
	return 5 // 5 MB max for world gen configs
}

func (v *WorldGenValidator) GetAllowedExtensions() []string {
	return []string{".json"}
}

func (v *WorldGenValidator) GetDescription() string {
	return "World Generation Config (.json, max 5 MB)"
}

func (v *WorldGenValidator) GetFileType() models.FileType {
	return models.FileTypeWorldGen
}

// ===== Helper Functions =====

// GetValidatorForFileType returns the appropriate validator for a file type
func GetValidatorForFileType(fileType models.FileType) (FileValidator, error) {
	switch fileType {
	case models.FileTypeResourcePack:
		return NewResourcePackValidator(), nil
	case models.FileTypeDataPack:
		return NewDataPackValidator(), nil
	case models.FileTypeServerIcon:
		return NewServerIconValidator(), nil
	case models.FileTypeWorldGen:
		return NewWorldGenValidator(), nil
	default:
		return nil, fmt.Errorf("unknown file type: %s", fileType)
	}
}

// CalculateSHA1 calculates the SHA1 hash of a file
func CalculateSHA1(file multipart.File) (string, error) {
	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate SHA1: %w", err)
	}

	// Reset file pointer
	file.Seek(0, 0)

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GetFileExtension returns the file extension
func GetFileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}
