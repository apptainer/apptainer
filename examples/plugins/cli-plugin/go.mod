module github.com/apptainer/apptainer/cli-example-plugin

go 1.16

require (
	github.com/apptainer/apptainer v0.0.0
	github.com/spf13/cobra v1.3.0
)

replace github.com/apptainer/apptainer => ./apptainer_source
