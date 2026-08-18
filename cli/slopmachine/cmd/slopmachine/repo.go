package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffss/cli/slopmachine/internal/store"
)

type repoProfileDocument struct {
	SchemaVersion    int                 `json:"schema_version"`
	RepoKey          string              `json:"repo_key"`
	Registered       bool                `json:"registered"`
	ForgeKind        string              `json:"forge_kind,omitempty"`
	TrustTier        string              `json:"trust_tier,omitempty"`
	VerifyCommand    string              `json:"verify_command,omitempty"`
	DeliveryMode     string              `json:"delivery_mode,omitempty"`
	Readiness        string              `json:"readiness,omitempty"`
	Bindings         map[string][]string `json:"bindings,omitempty"`
	ForgeReviewers   map[string]string   `json:"forge_reviewers,omitempty"`
	DryRun           bool                `json:"dry_run,omitempty"`
	ValidatedCommand string              `json:"validated_command,omitempty"`
}

func cmdRepo(st *store.Store, args []string, opts runOptions) int {
	verb := "show"
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		verb = args[0]
		args = args[1:]
	}
	switch verb {
	case "show", "register", "update", "unregister":
	default:
		return writeFailure(opts, 2, fmt.Errorf("unknown repo subcommand %q; use show, register, update, or unregister", verb))
	}
	fs, code := requireFlags("repo", args, opts)
	if code != 0 {
		return code
	}
	if verb == "show" || verb == "unregister" {
		for name := range fs {
			if name != "json" {
				return writeFailure(opts, 2, fmt.Errorf("repo %s does not accept --%s", verb, name))
			}
		}
	}
	if opts.dryRun && verb == "show" {
		return writeFailure(opts, 2, fmt.Errorf("--dry-run requires a mutating repo subcommand"))
	}
	key, err := resolveRepoKeyForOptions(st, opts)
	if err != nil {
		return writeFailure(opts, 2, err)
	}

	switch verb {
	case "show":
		profile, found, err := st.GetRepoProfile(key)
		if err != nil {
			return mapErr(err, opts)
		}
		return writeRepoProfile(profileDocument(key, profile, found), opts)
	case "unregister":
		if !opts.dryRun {
			if err := st.UnregisterRepoProfile(key); err != nil {
				return mapErr(err, opts)
			}
		}
		doc := profileDocument(key, machine.RepoProfile{}, false)
		markDryRun(&doc, opts)
		return writeRepoProfile(doc, opts)
	}

	// st is nil only for a dry run on a fresh installation; project against
	// the empty state the real command would create.
	var current machine.RepoProfile
	var found bool
	if st != nil {
		var err error
		current, found, err = st.GetRepoProfile(key)
		if err != nil {
			return mapErr(err, opts)
		}
	}
	if verb == "register" && found {
		return writeFailure(opts, 2, fmt.Errorf("%w: this repo is already registered; inspect it with slopmachine repo show or change it with slopmachine repo update", machine.ErrBadArgs))
	}
	if verb == "update" && !found {
		return mapErr(fmt.Errorf("%w: this repo has no profile; create one with slopmachine repo register", machine.ErrNotFound), opts)
	}
	profile := current
	if verb == "register" {
		profile = machine.RepoProfile{}
	}
	profile.RepoKey = key
	// Explicitly empty policy flags are rejected rather than treated as a
	// silent clear; only --bind documents empty-as-clear (replacement set).
	for _, name := range []string{"forge", "trust", "verify-cmd", "delivery", "readiness"} {
		if value, ok := fs[name]; ok && value == "" {
			return writeFailure(opts, 2, fmt.Errorf("--%s requires a non-empty value; re-register the profile to drop a policy field", name))
		}
	}
	if value, ok := fs["forge"]; ok {
		profile.ForgeKind = machine.ForgeKind(value)
	}
	if value, ok := fs["trust"]; ok {
		profile.TrustTier = machine.TrustTier(value)
	}
	if value, ok := fs["verify-cmd"]; ok {
		profile.VerifyCommand = value
	}
	if value, ok := fs["delivery"]; ok {
		profile.DeliveryMode = machine.DeliveryMode(value)
	}
	if value, ok := fs["readiness"]; ok {
		profile.Readiness = machine.Readiness(value)
	}
	if value, ok := fs["bind"]; ok {
		bindings, err := parseBindings(value)
		if err != nil {
			return writeFailure(opts, 2, err)
		}
		profile.Bindings = bindings
	}
	if value, ok := fs["forge-reviewer"]; ok {
		reviewers, err := parseForgeReviewers(value)
		if err != nil {
			return writeFailure(opts, 2, err)
		}
		profile.ForgeReviewers = reviewers
	}
	if err := machine.ValidateProfile(&profile); err != nil {
		return mapErr(err, opts)
	}
	// Declared review implementations must exist as reviewer identities, so a
	// bound profile can always satisfy the reviewer registry at release.
	if err := ensureReviewBindingsRegistered(st, profile); err != nil {
		return mapErr(err, opts)
	}
	if !opts.dryRun {
		var err error
		if verb == "register" {
			err = st.RegisterRepoProfile(profile)
		} else {
			err = st.UpdateRepoProfile(profile)
		}
		if err != nil {
			return mapErr(err, opts)
		}
	}
	doc := profileDocument(key, profile, true)
	markDryRun(&doc, opts)
	return writeRepoProfile(doc, opts)
}

// parseBindings decodes 'role=name,role=name' pairs; repeated roles
// accumulate and an empty value clears every binding.
func parseBindings(value string) (map[machine.Role][]string, error) {
	bindings := map[machine.Role][]string{}
	if strings.TrimSpace(value) == "" {
		return bindings, nil
	}
	for _, pair := range strings.Split(value, ",") {
		role, name, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || role == "" || name == "" {
			return nil, fmt.Errorf("%w: --bind expects comma-separated role=name pairs (roles: review, qa, venue, memory)", machine.ErrBadArgs)
		}
		bindings[machine.Role(role)] = append(bindings[machine.Role(role)], name)
	}
	return bindings, nil
}

// parseForgeReviewers decodes 'identity=login' pairs into the replacement
// forge-reviewer map; an empty value clears every mapping.
func parseForgeReviewers(value string) (map[string]string, error) {
	reviewers := map[string]string{}
	if strings.TrimSpace(value) == "" {
		return reviewers, nil
	}
	for _, pair := range strings.Split(value, ",") {
		identity, login, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || identity == "" || login == "" {
			return nil, fmt.Errorf("%w: --forge-reviewer expects comma-separated identity=login pairs (e.g. slopzapper=slopzapper[bot])", machine.ErrBadArgs)
		}
		if _, dup := reviewers[identity]; dup {
			return nil, fmt.Errorf("%w: --forge-reviewer maps %q twice", machine.ErrBadArgs, identity)
		}
		reviewers[identity] = login
	}
	return reviewers, nil
}

func ensureReviewBindingsRegistered(st *store.Store, profile machine.RepoProfile) error {
	bound := profile.Bindings[machine.RoleReview]
	if len(bound) == 0 && len(profile.ForgeReviewers) == 0 {
		return nil
	}
	// A nil store (dry run before any state exists) has an empty custom
	// registry; only built-ins can be bound.
	var registered []machine.ReviewerIdentity
	if st != nil {
		var err error
		registered, err = st.ListReviewers()
		if err != nil {
			return err
		}
	}
	allowed := make(map[string]struct{}, len(registered)+2)
	for _, reviewer := range machine.BuiltinReviewers() {
		allowed[string(reviewer)] = struct{}{}
	}
	for _, reviewer := range registered {
		allowed[string(reviewer)] = struct{}{}
	}
	for _, name := range bound {
		if _, ok := allowed[name]; !ok {
			if renamed := machine.LegacyReviewerRename(machine.ReviewerIdentity(name)); renamed != "" {
				return fmt.Errorf("%w: reviewer identity %q was renamed to %q", machine.ErrBadArgs, name, renamed)
			}
			return fmt.Errorf("%w: review binding %q is not a registered reviewer identity; register it first with slopmachine reviewers --add %s", machine.ErrBadArgs, name, name)
		}
	}
	for identity := range profile.ForgeReviewers {
		if _, ok := allowed[identity]; !ok {
			if renamed := machine.LegacyReviewerRename(machine.ReviewerIdentity(identity)); renamed != "" {
				return fmt.Errorf("%w: reviewer identity %q was renamed to %q", machine.ErrBadArgs, identity, renamed)
			}
			return fmt.Errorf("%w: forge reviewer %q is not a registered reviewer identity; register it first with slopmachine reviewers --add %s", machine.ErrBadArgs, identity, identity)
		}
	}
	return nil
}

func profileDocument(key string, profile machine.RepoProfile, registered bool) repoProfileDocument {
	doc := repoProfileDocument{SchemaVersion: 1, RepoKey: key, Registered: registered}
	if !registered {
		return doc
	}
	doc.ForgeKind = string(profile.ForgeKind)
	doc.TrustTier = string(profile.TrustTier)
	doc.VerifyCommand = profile.VerifyCommand
	doc.DeliveryMode = string(profile.DeliveryMode)
	doc.Readiness = string(profile.Readiness)
	if len(profile.Bindings) > 0 {
		doc.Bindings = make(map[string][]string, len(profile.Bindings))
		for role, names := range profile.Bindings {
			doc.Bindings[string(role)] = append([]string(nil), names...)
		}
	}
	if len(profile.ForgeReviewers) > 0 {
		doc.ForgeReviewers = make(map[string]string, len(profile.ForgeReviewers))
		for identity, login := range profile.ForgeReviewers {
			doc.ForgeReviewers[identity] = login
		}
	}
	return doc
}

func markDryRun(doc *repoProfileDocument, opts runOptions) {
	if opts.dryRun {
		doc.DryRun = true
		doc.ValidatedCommand = "repo"
	}
}

func writeRepoProfile(doc repoProfileDocument, opts runOptions) int {
	if opts.json {
		if err := writeJSON(doc); err != nil {
			return writeFailure(opts, 10, err)
		}
		return 0
	}
	prefix := "slopmachine repo"
	if doc.DryRun {
		prefix += " dry-run"
	}
	if !doc.Registered {
		fmt.Fprintf(os.Stdout, "%s registered=false\n", prefix)
		return 0
	}
	parts := []string{"registered=true"}
	appendPart := func(name, value string) {
		if value != "" {
			parts = append(parts, name+"="+value)
		}
	}
	appendPart("forge", doc.ForgeKind)
	appendPart("trust", doc.TrustTier)
	appendPart("delivery", doc.DeliveryMode)
	appendPart("readiness", doc.Readiness)
	if doc.VerifyCommand != "" {
		parts = append(parts, "verify="+shellQuoteCLI(doc.VerifyCommand))
	}
	if len(doc.Bindings) > 0 {
		roles := make([]string, 0, len(doc.Bindings))
		for role := range doc.Bindings {
			roles = append(roles, role)
		}
		slices.Sort(roles)
		bound := make([]string, 0, len(roles))
		for _, role := range roles {
			bound = append(bound, role+"="+strings.Join(doc.Bindings[role], "+"))
		}
		parts = append(parts, "bind="+strings.Join(bound, ";"))
	}
	if len(doc.ForgeReviewers) > 0 {
		identities := make([]string, 0, len(doc.ForgeReviewers))
		for identity := range doc.ForgeReviewers {
			identities = append(identities, identity)
		}
		slices.Sort(identities)
		pairs := make([]string, 0, len(identities))
		for _, identity := range identities {
			pairs = append(pairs, identity+"="+doc.ForgeReviewers[identity])
		}
		parts = append(parts, "forge-reviewers="+strings.Join(pairs, ";"))
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", prefix, strings.Join(parts, " "))
	return 0
}

func shellQuoteCLI(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
