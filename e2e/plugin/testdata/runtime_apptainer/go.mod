module github.com/apptainer/apptainer/e2e-cli-plugin

go 1.13

require (
	github.com/spf13/cobra v1.0.0
	github.com/apptainer/apptainer v0.0.0
)

replace github.com/apptainer/apptainer => ./apptainer_source
