package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/provider"
	repositorypkg "github.com/uinaf/ffss/cli/slopguard/internal/repository"
)

func runDoctor(ctx context.Context, arguments []string, stdout, stderr io.Writer, dependencies dependencies) int {
	jsonRequested, outputErr := doctorJSONRequested(arguments)
	if outputErr != nil {
		return writeDoctorResult(stdout, stderr, jsonRequested, doctorConfigFailure(""), 2)
	}
	flags := flag.NewFlagSet("slopguard doctor", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository or path within it")
	output := flags.String("output", "terminal", "output format: terminal or json")
	jsonOutput := flags.Bool("json", false, "print diagnostic as JSON")
	configFlags := bindConfigFlags(flags)
	if err := parseFlags(flags, arguments, stdout, io.Discard); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return writeDoctorResult(stdout, stderr, jsonRequested, doctorConfigFailure(providerName(configFlags)), 2)
	}
	explicitOutput := false
	flags.Visit(func(item *flag.Flag) { explicitOutput = explicitOutput || item.Name == "output" })
	if flags.NArg() != 0 || (*output != "terminal" && *output != "json") || (*jsonOutput && explicitOutput && *output != "json") {
		return writeDoctorResult(stdout, stderr, jsonRequested, doctorConfigFailure(providerName(configFlags)), 2)
	}
	jsonRequested = *jsonOutput || *output == "json"
	overrides, err := configFlags.overrides(flags)
	if err != nil {
		return writeDoctorResult(stdout, stderr, jsonRequested, doctorConfigFailure(providerName(configFlags)), 2)
	}
	repositoryContext, err := repositorypkg.Resolve(ctx, repositorypkg.Options{Path: *repository})
	if err != nil {
		return writeDoctorResult(stdout, stderr, jsonRequested, doctorTargetFailure(providerName(configFlags)), 2)
	}
	effective, err := config.Load(ctx, config.Options{
		Context:   repositoryContext,
		Overrides: overrides,
		LookupEnv: dependencies.lookupEnv,
		HomeDir:   dependencies.homeDir,
	})
	if err != nil {
		return writeDoctorResult(stdout, stderr, jsonRequested, doctorConfigFailure(providerName(configFlags)), 2)
	}
	doctor := dependencies.doctor
	if doctor == nil {
		doctor = provider.Doctor
	}
	diagnostic := doctor(ctx, provider.DoctorOptions{Repository: repositoryContext.Root(), Config: effective})
	exit := 1
	if diagnostic.FailureClass == protocol.FailureConfig {
		exit = 2
	} else if diagnostic.Status == provider.DoctorReady {
		exit = 0
	}
	return writeDoctorResult(stdout, stderr, jsonRequested, diagnostic, exit)
}

func doctorJSONRequested(arguments []string) (bool, error) {
	outputRequested, outputErr := reviewJSONRequested(arguments)
	jsonRequested := false
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			break
		}
		switch argument {
		case "--json", "-json":
			jsonRequested = true
		default:
			value, found := strings.CutPrefix(argument, "--json=")
			if !found {
				value, found = strings.CutPrefix(argument, "-json=")
			}
			if found {
				parsed, parseErr := strconv.ParseBool(value)
				if parseErr != nil {
					return true, parseErr
				}
				jsonRequested = parsed
			}
		}
		if reviewFlagConsumesNext(argument) && index+1 < len(arguments) {
			index++
		}
	}
	return outputRequested || jsonRequested, outputErr
}

func providerName(values configFlagValues) protocol.ProviderName {
	if values.engine == nil {
		return ""
	}
	name := protocol.ProviderName(*values.engine)
	switch name {
	case protocol.ProviderCodex, protocol.ProviderClaude, protocol.ProviderCursor, protocol.ProviderGrok:
		return name
	default:
		return ""
	}
}

func doctorTargetFailure(name protocol.ProviderName) provider.Diagnostic {
	return provider.Diagnostic{
		SchemaVersion:  provider.DoctorSchemaVersion,
		Status:         provider.DoctorNotReady,
		Provider:       name,
		Authentication: provider.AuthenticationDelegated,
		FailureClass:   protocol.FailureTarget,
		Message:        "repository path is not inside a Git worktree",
	}
}

func doctorConfigFailure(name protocol.ProviderName) provider.Diagnostic {
	return provider.Diagnostic{
		SchemaVersion:  provider.DoctorSchemaVersion,
		Status:         provider.DoctorNotReady,
		Provider:       name,
		Authentication: provider.AuthenticationDelegated,
		FailureClass:   protocol.FailureConfig,
		Message:        "provider configuration is invalid",
	}
}

func writeDoctorResult(stdout, stderr io.Writer, jsonOutput bool, diagnostic provider.Diagnostic, exit int) int {
	if err := diagnostic.Validate(); err != nil {
		diagnostic = doctorConfigFailure("")
		exit = 2
	}
	if jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(diagnostic); err != nil {
			report(stderr, "write doctor result: operation failed\n")
			return 2
		}
		return exit
	}
	if _, err := fmt.Fprintf(stdout, "status: %s\nprovider: %s version=%s compatible=%t\nweb_access=%t\nauthentication: %s\n",
		diagnostic.Status, diagnostic.Provider, diagnostic.Version, diagnostic.Compatible,
		diagnostic.WebAccess, diagnostic.Authentication,
	); err != nil {
		report(stderr, "write doctor result: operation failed\n")
		return 2
	}
	if diagnostic.Status == provider.DoctorNotReady {
		if _, err := fmt.Fprintf(stdout, "failure: %s: %s\n", diagnostic.FailureClass, diagnostic.Message); err != nil {
			report(stderr, "write doctor result: operation failed\n")
			return 2
		}
	}
	return exit
}
