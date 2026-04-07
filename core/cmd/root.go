package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"biene/internal/config"
	"biene/internal/server"
)

var (
	flagProfile   string
	flagHost      string
	flagPort      int
	flagWorkspace string
)

// Root is the top-level Cobra command.
var Root = &cobra.Command{
	Use:   "biene-core",
	Short: "Local core service for the Biene desktop app",
	Long: `biene-core runs the local core service used by the Biene desktop app.

Configuration is stored in ~/.biene/config.json.
Run 'biene-core config init' to create the config file.`,
	RunE: runServe,
}

func init() {
	Root.PersistentFlags().StringVar(&flagProfile, "profile", "", "Model profile to use (default: default_model)")
	Root.Flags().StringVar(&flagHost, "host", "127.0.0.1", "HTTP bind host")
	Root.Flags().IntVar(&flagPort, "port", 8080, "HTTP server port")
	Root.Flags().StringVar(&flagWorkspace, "workspace", "workspace", "Directory for agent workspaces (relative or absolute)")

	Root.AddCommand(configCmd)
}

// ─── Config sub-command ───────────────────────────────────────────────────

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage biene configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Run the interactive configuration wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return config.Init()
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
}

// ─── Serve ────────────────────────────────────────────────────────────────

func runServe(cmd *cobra.Command, args []string) error {
	loadResult, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if loadResult.Created {
		fmt.Printf("Config template created at %s\nEdit it or run 'biene-core config init', then restart.\n", loadResult.Path)
	}

	srv, err := server.New(server.Options{
		Host:          flagHost,
		Port:          flagPort,
		Config:        loadResult.Config,
		WorkspaceRoot: flagWorkspace,
	})
	if err != nil {
		return err
	}

	return srv.ListenAndServe()
}
