package output

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ArtifactCheatsheet   = "cheatsheet.md"
	ArtifactAliasesSH    = "aliases.suggested.sh"
	ArtifactAliasesFish  = "aliases.suggested.fish"
	ArtifactReportJSON   = "report.json"
	tempArtifactNamePart = ".tmp"
)

var knownArtifacts = map[string]struct{}{
	ArtifactCheatsheet:  {},
	ArtifactAliasesSH:   {},
	ArtifactAliasesFish: {},
	ArtifactReportJSON:  {},
}

func IsKnownArtifact(name string) bool {
	_, ok := knownArtifacts[name]
	return ok
}

func WriteArtifact(dir string, name string, data []byte) error {
	if !IsKnownArtifact(name) {
		return wrapError("write artifact", filepath.Join(dir, name), fmt.Errorf("unknown artifact name"))
	}

	finalPath := filepath.Join(dir, name)
	tempFile, err := os.CreateTemp(dir, "."+name+"-"+tempArtifactNamePart+"-")
	if err != nil {
		return wrapError("create temporary artifact", finalPath, err)
	}

	tempPath := tempFile.Name()
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(filePerm); err != nil && !permissionChangeUnsupported(err) {
		_ = tempFile.Close()
		return wrapError("set temporary artifact permissions", tempPath, err)
	}
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return wrapError("write temporary artifact", tempPath, err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return wrapError("sync temporary artifact", tempPath, err)
	}
	if err := tempFile.Close(); err != nil {
		return wrapError("close temporary artifact", tempPath, err)
	}
	if err := chmodFile(tempPath); err != nil {
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return wrapError("replace artifact", finalPath, err)
	}
	keepTemp = true
	if err := chmodFile(finalPath); err != nil {
		return err
	}
	return nil
}
