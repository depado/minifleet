package cmd

import "github.com/spf13/cobra"

var (
	sharedJSON bool
	sharedAll  bool
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
	c.PersistentFlags().String("path", "", "use this directory as the fleet target, bypassing known_fleets lookup")
	c.PersistentFlags().Bool("shallow", false, "use shallow clones (--depth 1 --filter=blob:none)")
	c.PersistentFlags().Int("concurrent", 0, "maximum concurrent operations (defaults to number of CPUs)")
}

func addUIFlags(c *cobra.Command) {
	c.PersistentFlags().String("interactive", "auto", "interactive mode: auto, always, never")
}

func addFormatFlag(c *cobra.Command) {
	c.PersistentFlags().BoolVar(&sharedJSON, "json", false, "output as JSON instead of table")
}

func addAllFlag(c *cobra.Command) {
	c.PersistentFlags().BoolVarP(&sharedAll, "all", "A", false, "operate on all known fleets, ignoring the current directory (--path takes precedence)")
}

func addPlanFlag(c *cobra.Command) {
	c.PersistentFlags().StringP("plan", "p", "", "load plan from YAML file (filters, command, fleet)")
}
