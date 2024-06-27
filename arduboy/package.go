package arduboy

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
