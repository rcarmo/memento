package main

import (
	"flag"
	"fmt"
	"github.com/rcarmo/memento/internal/assets"
	"io"
	"os"
)

type importOptions struct{ workspace, name, version, skillPath, zipPath string }

func parseImportArgs(args []string, stderr io.Writer) (importOptions, error) {
	flags := flag.NewFlagSet("memento-skill-import-go", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := importOptions{}
	flags.StringVar(&options.workspace, "workspace", ".", "workspace root")
	flags.StringVar(&options.name, "name", "", "skill name")
	flags.StringVar(&options.version, "version", "", "skill version")
	flags.StringVar(&options.skillPath, "skill-md", "", "SKILL.md path")
	flags.StringVar(&options.zipPath, "zip", "", "skill ZIP path")
	if err := flags.Parse(args); err != nil {
		return importOptions{}, err
	}
	if flags.NArg() != 0 || options.name == "" || options.version == "" || options.skillPath == "" || options.zipPath == "" {
		return importOptions{}, fmt.Errorf("--name, --version, --skill-md and --zip are required")
	}
	return options, nil
}
func run(args []string, stdout, stderr io.Writer) int {
	options, err := parseImportArgs(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "memento-skill-import-go:", err)
		return 2
	}
	skillMD, err := os.ReadFile(options.skillPath)
	if err != nil {
		fmt.Fprintln(stderr, "memento-skill-import-go:", err)
		return 1
	}
	raw, err := os.ReadFile(options.zipPath)
	if err != nil {
		fmt.Fprintln(stderr, "memento-skill-import-go:", err)
		return 1
	}
	destination, err := assets.ImportSkillPack(options.workspace, options.name, options.version, string(skillMD), raw)
	if err != nil {
		fmt.Fprintln(stderr, "memento-skill-import-go:", err)
		return 1
	}
	fmt.Fprintln(stdout, destination)
	return 0
}

var exit = os.Exit

func main() { exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
