// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/apptainer/apptainer/cmd/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

/*
 * This tool aims at gathering the list of Apptainer commands and optionally
 * the list of commands covered by the E2E tests. When gathering the list of
 * E2E tests, we also calculate the coverage compared to all the Apptainer
 * commands.
 *
 * Note that the list of all the commands has some limitations. Since we create
 * the list with Cobra, the following assumptions are made:
 * - Commands are based on the following scheme (which is not matching reality at 100%):
 *		apptainer <cmd> [[--<cmd-opt>] [<sub-cmd> [--<subcmd-opt]]]
 * - Cobra does not actually create a tree of commands/sub-commands/options but rather
 * a list of commands and sub-list of sub-commands and options. As a result, it is not
 * possible to track sub-command that are specific to a command option; and therefore
 * not possible to get all the actual Apptainer commands. For instance, Apptainer
 * support commands such as:
 *		apptainer remote --config <configfile> add --global <name> <uri>
 * As the code stands, it is not possible to capture the fact that the config option
 * alloows one to use the add sub-command.
 * - the e2e tests are using the long version of the option, not the short version. At
 * the moment, we have no way to associate short and long versions of options.
 */

// Global variable to track the total number of tested commands.
var tested = 0

type apptainerCmd struct {
	Name     string         `json:"name"`
	Options  []string       `json:"options"`
	Children []apptainerCmd `json:"children"`
}

func checkCmd(sCmd string, e2eCmds string, resultFile *os.File, verbose bool) error {
	currentCmd := sCmd
	currentOpt := ""
	if strings.Index(sCmd, "--") > 0 {
		cmdElts := strings.Split(sCmd, "--")
		if len(cmdElts) != 2 {
			return fmt.Errorf("wrong number of options: %d", len(cmdElts))
		}
		currentCmd = strings.Trim(cmdElts[0], " ")
		currentOpt = strings.Trim("--"+cmdElts[1], " ")
	}

	isTested := false
	for _, e2eCmd := range strings.Split(e2eCmds, "\n") {
		e2eCmd = strings.Trim(e2eCmd, " ")
		if e2eCmd == "" {
			continue
		}

		if e2eCmd != "" && strings.Contains(e2eCmd, currentCmd) {
			if currentOpt == "" || (currentOpt != "" && strings.Contains(e2eCmd, currentOpt)) {
				isTested = true
				break
			}
		}
	}

	str := fmt.Sprintf("UNTESTED: %s\n", sCmd)
	if isTested {
		if verbose {
			fmt.Printf("%s%s%s\n", "\x1b[32m", sCmd, "\x1b[0m")
		}
		str = fmt.Sprintf("TESTED: %s\n", sCmd)
		tested++
	} else {
		if verbose {
			fmt.Printf("%s%s%s\n", "\x1b[31m", sCmd, "\x1b[0m")
		}
	}
	resultFile.WriteString(str)

	return nil
}

func loadData(apptainerCmdsFile, e2eCmdsFile string) (string, string, error) {
	e2eData, err := ioutil.ReadFile(e2eCmdsFile)
	if err != nil {
		return "", "", err
	}
	e2eCmds := string(e2eData)

	// Strip ` --debug` (and ` -d` just in case) from commands
	// This is needed as the generated command tree doesn't have anything ahead
	// of the command name, so --debug will stop a match happening
	e2eCmds = strings.ReplaceAll(e2eCmds, "apptainer --debug", "apptainer")
	e2eCmds = strings.ReplaceAll(e2eCmds, "apptainer -d", "apptainer")

	apptainerData, err := ioutil.ReadFile(apptainerCmdsFile)
	if err != nil {
		return "", "", err
	}
	apptainerCmds := string(apptainerData)

	return apptainerCmds, e2eCmds, nil
}

func analyseData(apptainerCmds string, e2eCmds string, report string, verbose bool) (string, error) {
	var (
		err        error
		resultFile *os.File
	)
	if report == "" {
		resultFile, err = ioutil.TempFile("", "apptainer-cmd-coverage-")
	} else {
		resultFile, err = os.OpenFile(report, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	}
	if err != nil {
		return "", fmt.Errorf("failed to create file to store coverage results: %s", err)
	}
	defer resultFile.Close()

	for _, apptainerCmd := range strings.Split(apptainerCmds, "\n") {
		if apptainerCmd != "" {
			err = checkCmd(apptainerCmd, e2eCmds, resultFile, verbose)
			if err != nil {
				return "", fmt.Errorf("failed to check cmd %s", apptainerCmd)
			}
		}
	}

	totalCmds := len(strings.Split(apptainerCmds, "\n"))
	ratio := tested * 100 / totalCmds
	fmt.Printf("E2E cmd coverage: %d/%d (%d%%)\n", tested, totalCmds, ratio)
	return resultFile.Name(), nil
}

func parseCmd(cmd apptainerCmd, outputFile *os.File) error {
	str := fmt.Sprintf("%s", cmd)

	// When creating the tree of commands, it includes elements such as:
	//		{apptainer cache [help] [{apptainer cache clean [force help name type] []} {apptainer cache list [help type verbose] []}]}
	// The fact that the element includes "[{" means that the first part is a command to
	// consider (here, "apptainer cache [help]"), while the rest of the element is
	// a summary of commands already listed independently. This type of element is the
	// result of reaching to bottom of a tree branch.
	if strings.Contains(str, "[{") {
		str = strings.Split(str, "[{")[0]
	}
	cmds := strings.Split(str, "apptainer")
	for _, cmdStr := range cmds { // This may be a command with sub-commands
		if cmdStr != "{" { // Going down the tree, we may be at the bottom with an empty list

			// We clean up the command
			cmdStr = strings.Replace(cmdStr, "[]}", "", -1)
			cmdStr = strings.Replace(cmdStr, "{", "", -1)
			cmdStr = strings.Replace(cmdStr, "]}", "", -1)
			cmdStr = strings.Replace(cmdStr, "] [", "]", -1)

			// We check whether the command has options to handle
			subcmds := strings.Split(cmdStr, "[")
			if len(subcmds) == 2 { // Options are associated to the command
				opts := subcmds[1]
				opts = strings.Replace(opts, "]", "", -1)
				// Handle each option separately
				for _, opt := range strings.Split(opts, " ") {
					if strings.Trim(opt, " \t") != "" {
						// Command with option
						str = fmt.Sprintf("apptainer %s --%s\n", strings.Trim(subcmds[0], " \t"), strings.Trim(opt, " \t"))
						outputFile.WriteString(str)
					}
				}
			} else {
				// Command without option or sub-commands
				str = fmt.Sprintf("apptainer %s\n", cmdStr)
				outputFile.WriteString(str)
			}
		}
	}

	return nil
}

func (c *apptainerCmd) addCmd(cmd apptainerCmd, outputFile *os.File) error {
	c.Children = append(c.Children, cmd)
	return parseCmd(cmd, outputFile)
}

func (c *apptainerCmd) addOpt(opt string) {
	c.Options = append(c.Options, opt)
}

func buildTree(root *cobra.Command, outputFile *os.File) apptainerCmd {
	root.InitDefaultHelpFlag()

	tree := apptainerCmd{Name: root.CommandPath(), Options: nil, Children: nil}

	root.Flags().VisitAll(func(flag *pflag.Flag) {
		tree.addOpt(flag.Name)
	})

	for _, c := range root.Commands() {
		tree.addCmd(buildTree(c, outputFile), outputFile)
	}

	return tree
}

func createCmdsFiles() (string, string, error) {
	jsonFile, err := ioutil.TempFile("", "apptainerCmdsJSON-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create command JSON file: %s", err)
	}
	defer jsonFile.Close()

	textFile, err := ioutil.TempFile("", "apptainerCmds-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create command text file: %s", err)
	}
	defer textFile.Close()

	// iniatialize apptainer CLI by registering all commands without plugins
	cli.Init(false)
	cli.RootCmd().InitDefaultHelpCmd()
	cli.RootCmd().InitDefaultVersionFlag()

	tree := buildTree(cli.RootCmd(), textFile)

	json, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal data: %s", err)
	}

	jsonFile.WriteString(string(json))

	return jsonFile.Name(), textFile.Name(), nil
}

func main() {
	verbose := flag.Bool("v", false, "Enable verbose mode")
	coverage := flag.String("coverage", "", "Use specified coverage file from e2e-test to produce a cmd coverage report")
	report := flag.String("report", "", "Specify location for cmd coverage report")

	flag.Parse()

	jsonFilePath, textFilePath, err := createCmdsFiles()
	if err != nil {
		log.Fatalf("failed to gather all the Apptainer commands: %s", err)
	}

	resultFilePath := ""
	if *coverage != "" {
		apptainerCmds, e2eCmds, err := loadData(textFilePath, *coverage)
		if err != nil {
			log.Fatalf("failed to load e2e-test coverage data: %s", err)
		}
		resultFilePath, err = analyseData(apptainerCmds, e2eCmds, *report, *verbose)
		if err != nil {
			log.Fatalf("failed to analyze e2e-test coverage data: %s", err)
		}
	}

	if *verbose {
		fmt.Printf("List of all the Apptainer commands (JSON) is in: %s\n", jsonFilePath)
		fmt.Printf("List of all the Apptainer commands (text) is in: %s\n", textFilePath)
	}
	if *coverage != "" {
		fmt.Printf("E2E coverage report saved in: %s\n", resultFilePath)
	}
}
