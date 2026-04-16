package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"cmd-mint/internal/model"
)

const detectSampleLimit = 64 * 1024

func ParseFile(source model.HistorySource) (Result, error) {
	shell := source.SourceShell
	if shell == model.ShellAuto {
		detected, err := DetectShellFile(source.SourceFile)
		if err != nil {
			return Result{}, err
		}
		shell = detected
	}

	switch shell {
	case model.ShellBash:
		return ParseBashFile(source.SourceFile)
	case model.ShellZsh:
		return ParseZshFile(source.SourceFile)
	case model.ShellFish:
		return ParseFishFile(source.SourceFile)
	default:
		return Result{}, fmt.Errorf("unsupported shell %q", source.SourceShell)
	}
}

func DetectShellFile(path string) (model.Shell, error) {
	file, err := os.Open(path)
	if err != nil {
		return model.ShellAuto, err
	}
	defer file.Close()

	return DetectShell(file)
}

func DetectShell(r io.Reader) (model.Shell, error) {
	var sample bytes.Buffer
	_, err := io.CopyN(&sample, r, detectSampleLimit)
	if err != nil && err != io.EOF {
		return model.ShellAuto, err
	}
	return detectShellSample(sample.String()), nil
}

func detectShellSample(sample string) model.Shell {
	scanner := bufio.NewScanner(strings.NewReader(sample))
	sawBashTimestamp := false
	sawPlainCommand := false

	for scanner.Scan() {
		line := trimLineEnding(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		if _, _, status := parseZshEntry(line); status == model.ParseStatusParsed && looksLikeZshExtendedHistory(line) {
			return model.ShellZsh
		}
		if _, ok := parseFishCommandLine(line); ok {
			return model.ShellFish
		}
		if _, ok := parseBashTimestampMarker(line); ok {
			sawBashTimestamp = true
			continue
		}
		sawPlainCommand = true
	}

	if sawBashTimestamp || sawPlainCommand {
		return model.ShellBash
	}
	return model.ShellBash
}
