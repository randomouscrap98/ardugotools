package arduboy

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"slices"
	"strings"

	_ "image/gif"
	_ "image/png"
)

const (
	PackageInfoFile = "info.json"
)

// All data parsed from info.json. We don't parse all fields, only the
// ones we actually care about here.
type PackageInfo struct {
	SchemaVersion int              `json:"schemaVersion"`
	Title         string           `json:"title"`
	Description   string           `json:"description"`
	Author        string           `json:"author"`
	Version       string           `json:"version"`
	Binaries      []*PackageBinary `json:"binaries"`
}

type PackageBinary struct {
	Title      string `json:"title"`
	Filename   string `json:"filename"`
	Device     string `json:"device"`
	TitleImage string `json:"cartimage"`
	FlashData  string `json:"flashdata"`
	FlashSave  string `json:"flashsave"`
}

func ReadPackageInfo(archive *zip.ReadCloser) (PackageInfo, error) {
	var result PackageInfo
	for _, f := range archive.File {
		if strings.ToLower(f.Name) == PackageInfoFile {
			jsonreader, err := f.Open()
			if err != nil {
				return result, err
			}
			defer jsonreader.Close()
			jsonraw, err := io.ReadAll(jsonreader)
			if err != nil {
				return result, err
			}
			err = json.Unmarshal(jsonraw, &result)
			if err != nil {
				return result, err
			}
			return result, nil
		}
	}
	return result, fmt.Errorf("Couldn't find %s in package", PackageInfoFile)
}

// Read the entirety of the given package file into memory and return it
func LoadPackageFile(archive *zip.ReadCloser, filename string) ([]byte, error) {
	for _, f := range archive.File {
		// Assume they're using windows and thus have no case sensitivity
		if strings.ToLower(f.Name) == filename {
			reader, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			return io.ReadAll(reader)
		}
	}
	return nil, fmt.Errorf("File not found")
}

// Scan package for the first image alphabetically which matches the aspect ratio.
func FindSuitablePackageImage(archive *zip.ReadCloser) (string, error) {
	images := make([]string, 0)
	for _, f := range archive.File {
		checkname := strings.ToLower(f.Name)
		// Assume they're using windows and thus have no case sensitivity
		if strings.HasSuffix(checkname, ".png") || strings.HasSuffix(checkname, ".gif") {
			ireader, err := f.Open()
			if err != nil {
				return "", err
			}
			defer ireader.Close()
			info, _, err := image.DecodeConfig(ireader)
			if err != nil {
				return "", err
			}
			// this is exactly the ratio we're looking for
			if float64(info.Width)/float64(info.Height) == float64(ScreenWidth)/float64(ScreenHeight) {
				images = append(images, f.Name)
			}
		}
	}
	if len(images) == 0 {
		return "", fmt.Errorf("No suitable image found in archive")
	}
	slices.Sort(images)
	return images[0], nil
}

// func GetPackageReader(archive *zip.ReadCloser, filename string) ([]byte, error) {
//   archive.
// }
