// Command heatseeker is the single binary of the core service.
//
//	heatseeker api            HTTP API
//	heatseeker worker         asynq workers + scheduler
//	heatseeker migrate [cmd]  goose: up (default) | down | redo | status | version
//	heatseeker gen [what]     openapi | shared | all (default)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // schedules follow IANA zones; the static binary may run without /usr/share/zoneinfo

	"heatseeker/api/internal/gen"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/logger"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	if len(argv) < 1 {
		usage()
		return 2
	}
	cmd, args := argv[0], argv[1:]

	// gen does not need configuration or infrastructure.
	if cmd == "gen" {
		if err := runGen(args); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		return 0
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		return 1
	}
	log := logger.New(cfg.App.LogLevel, cfg.App.Env)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "api":
		err = runAPI(ctx, cfg, log)
	case "worker":
		err = runWorker(ctx, cfg, log)
	case "migrate":
		err = runMigrate(ctx, cfg, log, args)
	default:
		usage()
		return 2
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("fatal", "err", err)
		return 1
	}
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: heatseeker <api|worker|migrate|gen> [args]")
}

func runGen(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	openapiOut := fs.String("openapi-out", "openapi/openapi.json", "where to write the OpenAPI document")
	sharedOut := fs.String("shared-out", "../../packages/shared/src/generated", "directory for generated TypeScript")
	if err := fs.Parse(args); err != nil {
		return err
	}
	what := "all"
	if fs.NArg() > 0 {
		what = fs.Arg(0)
	}
	switch what {
	case "openapi":
		return gen.OpenAPI(*openapiOut)
	case "shared":
		return gen.Shared(*sharedOut)
	case "all":
		if err := gen.OpenAPI(*openapiOut); err != nil {
			return err
		}
		return gen.Shared(*sharedOut)
	default:
		return fmt.Errorf("unknown gen target %q (openapi|shared|all)", what)
	}
}
