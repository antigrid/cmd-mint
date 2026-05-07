package output

import (
	"path/filepath"

	"cmd-mint/internal/model"
)

type ArtifactOptions struct {
	NoAliasFile bool
	MaxAliases  int
	JSON        bool
	Shell       model.Shell
}

type Artifact struct {
	Name string
	Data []byte
}

func RenderAliasAndJSONArtifacts(report model.Report, options ArtifactOptions) ([]Artifact, error) {
	var artifacts []Artifact

	if !options.NoAliasFile {
		artifacts = append(artifacts, Artifact{
			Name: ArtifactAliasesSH,
			Data: RenderAliasesSH(report.AliasSuggestions, options.MaxAliases),
		})
		if shouldRenderFishAliases(report, options.Shell) {
			artifacts = append(artifacts, Artifact{
				Name: ArtifactAliasesFish,
				Data: RenderAliasesFish(report.AliasSuggestions, options.MaxAliases),
			})
		}
	}

	if options.JSON {
		data, err := RenderReportJSON(report)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{
			Name: ArtifactReportJSON,
			Data: data,
		})
	}

	return artifacts, nil
}

func WriteAliasAndJSONArtifacts(dir string, report model.Report, options ArtifactOptions) ([]string, error) {
	artifacts, err := RenderAliasAndJSONArtifacts(report, options)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if err := WriteArtifact(dir, artifact.Name, artifact.Data); err != nil {
			return nil, err
		}
		paths = append(paths, filepath.Join(dir, artifact.Name))
	}
	return paths, nil
}

func shouldRenderFishAliases(report model.Report, requestedShell model.Shell) bool {
	if requestedShell == model.ShellFish {
		return true
	}
	for _, source := range report.Sources {
		if source.SourceShell == model.ShellFish {
			return true
		}
	}
	return false
}
