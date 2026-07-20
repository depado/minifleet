package cmd

import "github.com/spf13/cobra"

var (
	sharedFormat string
	sharedAll    bool
)

func addConfigurationFlag(c *cobra.Command) {
	c.PersistentFlags().StringP("conf", "c", "", "configuration file to use")
}

func addLoggerFlags(c *cobra.Command) {
	c.PersistentFlags().String("log.level", "info", "one of debug, info, warn, error")
	c.PersistentFlags().String("log.format", "text", "one of json or text")
	c.PersistentFlags().Bool("log.source", false, "display source file and line of log call")
	c.PersistentFlags().String("log.color", "auto", "colorized output: auto, always, never")
}

func addGitHubFlags(c *cobra.Command) {
	c.PersistentFlags().String("github.token", "", "GitHub personal access token ($GITHUB_TOKEN)")
	c.PersistentFlags().String("github.host", "github.com", "GitHub host (use a custom host for GitHub Enterprise)")
}

func addFleetFlags(c *cobra.Command) {
	c.PersistentFlags().String("fleet.path", "", "override clone directory, bypass host/owner nesting")
	c.PersistentFlags().Bool("fleet.shallow", false, "use shallow clones (--depth 1 --filter=blob:none)")
	c.PersistentFlags().Int("fleet.concurrent", 5, "maximum concurrent operations")
}

func addUIFlags(c *cobra.Command) {
	c.PersistentFlags().Bool("ui.progress", true, "show progress bars")
	c.PersistentFlags().Bool("ui.color", true, "enable colored output")
}

func addFormatFlag(c *cobra.Command) {
	c.PersistentFlags().StringVarP(&sharedFormat, "format", "f", "table", "output format (table, json, yaml)")
}

func addAllFlag(c *cobra.Command) {
	c.PersistentFlags().BoolVarP(&sharedAll, "all", "A", false, "operate on all known fleets, ignoring the current directory")
}

func addPlanFlag(c *cobra.Command) {
	c.PersistentFlags().StringP("plan", "p", "", "load plan from YAML file (filters, command, fleet)")
}
