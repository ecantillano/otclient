package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	launcher "github.com/ecantillano/otclient/launcher"
)

var buildVersion = launcher.LauncherVersion

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }

func (values *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("host cannot be empty")
	}
	*values = append(*values, value)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Thappy Launcher:", launcher.PublicError(err))
		os.Exit(1)
	}
}

func run(arguments []string) error {
	defaultInstall, err := launcher.DefaultInstallDir()
	if err != nil {
		return err
	}

	flags := flag.NewFlagSet("thappy-launcher", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	installDir := flags.String("install-dir", defaultInstall, "Thappy installation directory")
	configPath := flags.String("config", "", "launcher configuration JSON")
	channelOverride := flags.String("channel", "", "release channel: stable or test")
	manifestOverride := flags.String("manifest-url", "", "HTTPS manifest URL")
	clientOverride := flags.String("client", "", "relative client executable path")
	noLaunch := flags.Bool("no-launch", false, "update without starting Thappy")
	diagnose := flags.Bool("diagnose", false, "print a redacted diagnostic report and exit")
	showVersion := flags.Bool("version", false, "print launcher version and exit")
	var extraHosts stringList
	flags.Var(&extraHosts, "allow-host", "additional exact HTTPS host; repeatable")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *showVersion {
		fmt.Println(buildVersion)
		return nil
	}

	configExplicit := *configPath != ""
	if *configPath == "" {
		*configPath = filepath.Join(*installDir, "launcher-config.json")
	}
	config := launcher.DefaultConfig()
	loaded, loadErr := launcher.LoadConfig(*configPath)
	if loadErr == nil {
		config = loaded
	} else if configExplicit || !errors.Is(loadErr, os.ErrNotExist) {
		return fmt.Errorf("load config: %w", loadErr)
	}
	config.AllowedHosts = append(config.AllowedHosts, extraHosts...)
	if err := config.Validate(); err != nil {
		return err
	}

	channel := config.DefaultChannel
	if *channelOverride != "" {
		channel = *channelOverride
	}
	if channel != "stable" && channel != "test" {
		return fmt.Errorf("invalid channel %q", channel)
	}
	manifestURL := *manifestOverride
	if manifestURL == "" {
		manifestURL, _ = config.ManifestURL(channel)
	} else if err := launcher.ValidateHTTPSURL(manifestURL, config.AllowedHosts); err != nil {
		return fmt.Errorf("manifest URL: %w", err)
	}

	updater, err := launcher.NewUpdater(*installDir, config, nil)
	if err != nil {
		return err
	}
	updater.LauncherVersion = buildVersion
	updater.Progress = func(progress launcher.ProgressEvent) {
		switch progress.Phase {
		case "planned":
			fmt.Fprintf(os.Stderr, "Thappy update: %d bytes in changed components.\n", progress.TotalBytes)
		case "downloading", "downloaded":
			fmt.Fprintf(os.Stderr, "Thappy update: %s %s (%d/%d bytes).\n", progress.Phase, progress.Component, progress.CompletedBytes, progress.TotalBytes)
		case "applying":
			fmt.Fprintln(os.Stderr, "Thappy update: applying verified files.")
		}
	}
	clientExecutable := *clientOverride
	if clientExecutable == "" {
		clientExecutable = config.ClientExecutables[runtime.GOOS]
	}
	if *diagnose {
		return launcher.WriteDiagnostic(launcher.BuildDiagnostic(updater, channel, manifestURL, clientExecutable))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	runner := &launcher.Runner{
		Updater:          updater,
		Channel:          channel,
		ManifestURL:      manifestURL,
		ClientExecutable: clientExecutable,
		ClientArgs:       config.ClientArgs,
		NoLaunch:         *noLaunch,
	}
	result, err := runner.Run(ctx)
	if err != nil {
		return err
	}
	if result.Offline {
		fmt.Fprintln(os.Stderr, "Update unavailable. Starting the last verified client offline.")
	} else if result.Update.Status == launcher.UpdateApplied {
		fmt.Printf("Updated Thappy from %s to %s.\n", result.Update.PreviousVersion, result.Update.InstalledVersion)
	}
	return nil
}
