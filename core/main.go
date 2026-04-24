package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"tinte/internal/config"
	"tinte/internal/logging"
	"tinte/internal/server"
)

func main() {
	host := flag.String("host", "127.0.0.1", "HTTP bind host")
	port := flag.Int("port", 8080, "HTTP server port")
	workspace := flag.String("workspace", "workspace", "Directory for agent workspaces (relative or absolute)")
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %s\n", strings.Join(flag.Args(), " "))
		flag.Usage()
		os.Exit(2)
	}

	logPath := logging.Init()
	defer logging.Close()
	slog.Info("core starting", "host", *host, "port", *port, "workspace", *workspace, "log_file", logPath)

	loadResult, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("loading config: %w", err))
		os.Exit(1)
	}

	if loadResult.Created {
		fmt.Printf("Config template created at %s\nEdit it, then restart.\n", loadResult.Path)
	}

	srv, err := server.New(server.Options{
		Host:          *host,
		Port:          *port,
		Config:        loadResult.Config,
		WorkspaceRoot: *workspace,
		AuthToken:     os.Getenv("TINTE_CORE_TOKEN"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
