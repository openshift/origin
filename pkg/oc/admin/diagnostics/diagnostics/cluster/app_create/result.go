package app_create

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (d *AppCreate) logResult() {
	// start with some Info logs to the user
	if d.result.App.Required {
		d.out.Info("DCluAC48", fmt.Sprintf("App creation and readiness completed with success=%t in %v", d.result.App.Success, d.result.App.TotalDuration))
	}
	if d.result.Service.Required {
		d.out.Info("DCluAC49", fmt.Sprintf("Service creation and testing completed with success=%t in %v", d.result.Service.Success, d.result.Service.TotalDuration))
	}
	if d.result.Route.Required {
		d.out.Info("DCluAC50", fmt.Sprintf("Route creation and testing completed with success=%t in %v", d.result.Route.Success, d.result.Route.TotalDuration))
	}
	d.out.Info("DCluAC51", fmt.Sprintf("Entire create/test completed with success=%t in %v", d.result.Success, d.result.TotalDuration))

	// check whether results are supposed to be written to disk at all
	if d.writeResultDir != "" {
		// create the write directory if needed
		if err := os.MkdirAll(d.writeResultDir, os.ModePerm); err != nil {
			d.out.Warn("DCluAC036", err, fmt.Sprintf("Could not create debug output directory '%s': %v", d.writeResultDir, err))
			return
		}
	} else {
		d.out.Debug("DCluAC037", "No output directory specified; results will not be written to files.")
		return
	}

	// write the result struct itself
	filename := filepath.Join(d.writeResultDir, "result.json")
	file, err := os.Create(filename)
	if err != nil {
		d.out.Warn("DCluAC038", err, fmt.Sprintf("Could not create result output file '%s': %v", filename, err))
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(d.result)
	if err != nil {
		d.out.Warn("DCluAC039", err, fmt.Sprintf("Could not write results to output file '%s': %v", filename, err))
		return
	}
}
