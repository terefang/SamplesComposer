package xoscillator

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// presetsFS embeds all .preset files located in the current Go package directory.
//
//go:embed presets/*.preset
var presetsFS embed.FS

// LoadEmbeddedSynthPresets reads every embedded .preset file and unmarshals it into a []SynthParams slice.
func LoadEmbeddedSynthPresets() ([]SynthParams, error) {
	// List all entries in the embedded filesystem root.
	entries, err := fs.ReadDir(presetsFS, "presets")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded presets directory: %w", err)
	}

	// Collect .preset filenames and sort them deterministically.
	var fileNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			fileNames = append(fileNames, entry.Name())
		}
	}
	sort.Strings(fileNames)

	// Unmarshal each embedded .preset file into a SynthParams struct.
	presets := make([]SynthParams, 0, len(fileNames))
	for _, name := range fileNames {
		// Read raw JSON bytes from the embedded file.
		raw, err := fs.ReadFile(presetsFS, "presets/"+name)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded preset %q: %w", name, err)
		}

		// Unmarshal JSON into a SynthParams struct.
		var preset SynthParams
		if err := json.Unmarshal(raw, &preset); err != nil {
			return nil, fmt.Errorf("failed to unmarshal embedded preset %q: %w", name, err)
		}

		// Append the decoded preset to the slice.
		presets = append(presets, preset)
	}

	// Return the slice of SynthParams structs.
	return presets, nil
}

// FindPresetByName searches presets for an entry matching name and returns it along with a found flag.
func FindPresetByName(name string, presets []SynthParams) (SynthParams, bool) {
	// Iterate over every preset in the provided slice.
	for _, p := range presets {
		// Return the matching SynthParams and true when Name matches.
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}

	// Return a zero SynthParams struct and false when no match is found.
	return SynthParams{}, false
}
