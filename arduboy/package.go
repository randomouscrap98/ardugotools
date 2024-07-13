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
	Title     string `json:"title"`
	Filename  string `json:"filename"`
	Device    string `json:"device"`
	CartImage string `json:"cartimage"`
	FlashData string `json:"flashdata"`
	FlashSave string `json:"flashsave"`
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
	datareader, err := archive.Open(filename)
	if err != nil {
		return nil, err
	}
	defer datareader.Close()
	return io.ReadAll(datareader)
}

// Scan package for the first image alphabetically which matches the aspect ratio.
func FindSuitablePackageImage(archive *zip.ReadCloser) (string, error) {
	images := make([]string, 0)
	for _, f := range archive.File {
		if strings.Contains(f.Name, "/") {
			continue // Don't look at folders deeper than root
		}
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

// Search for a suitable binary within the archive. We try to find a matching
// binary using either device or title. If neither are set, the only way this function
// works is if there's only one option.
func FindSuitableBinary(info *PackageInfo, device string, title string) (*PackageBinary, error) {
	var binaries []*PackageBinary
	var bnames []string

	if len(info.Binaries) == 1 && device == "" && title == "" {
		binaries = info.Binaries
	} else {
		binaries = make([]*PackageBinary, 0)
		for _, pb := range info.Binaries {
			if strings.ToLower(device) == strings.ToLower(pb.Device) ||
				strings.ToLower(title) == strings.ToLower(pb.Title) {
				binaries = append(binaries, pb)
				bnames = append(bnames, pb.Title)
			}
		}
	}

	if len(binaries) == 0 {
		return nil, fmt.Errorf("No matching binary")
	} else if len(binaries) != 1 {
		return nil, fmt.Errorf("Multiple matching binaries in package: %s", strings.Join(bnames, ","))
	}

	return binaries[0], nil
}

// A rather dangerous function to find binaries: just get the first binary that matches
// ANY of the given devices. The order of the devices doesn't matter, just the order
// of the binaries in the package
func FindAnyBinary(info *PackageInfo, device []string) (*PackageBinary, error) {
	for _, pb := range info.Binaries {
		for _, dv := range device {
			if strings.ToLower(dv) == strings.ToLower(pb.Device) {
				return pb, nil
			}
		}
	}
	return nil, fmt.Errorf("No matching binary")
}

// func GetPackageReader(archive *zip.ReadCloser, filename string) ([]byte, error) {
//   archive.
// }
