package main

import (
	"bytes"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fatih/color"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"github.com/shyiko/kubensx/cli"
	nsx "github.com/shyiko/kubensx/context"
	nsxkubectl "github.com/shyiko/kubensx/context/kubectl"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
	surveyterminal "gopkg.in/AlecAivazis/survey.v1/terminal"
	"os"
	"regexp"
	"sort"
	"strings"
)

var version string

func init() {
	log.SetFormatter(&simpleFormatter{})
	log.SetLevel(log.InfoLevel)
}

type simpleFormatter struct{}

func (f *simpleFormatter) Format(entry *log.Entry) ([]byte, error) {
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "%s ", entry.Message)
	for k, v := range entry.Data {
		fmt.Fprintf(b, "%s=%+v ", k, v)
	}
	b.Truncate(b.Len() - 1)
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func init() {
	// remove "? " prefix
	survey.SelectQuestionTemplate = strings.Replace(survey.SelectQuestionTemplate,
		`{{- color "green+hb"}}{{ QuestionIcon }} {{color "reset"}}`, "", 1)
	survey.SelectQuestionTemplate = strings.Replace(survey.SelectQuestionTemplate,
		`{{.Answer}}`, `{{or .Answer "\"\""}}`, 1)
	survey.SelectQuestionTemplate = strings.Replace(survey.SelectQuestionTemplate,
		`{{- $choice}}`, `{{- or $choice "\"\""}}`, 1)
	survey.SelectQuestionTemplate = strings.Replace(survey.SelectQuestionTemplate,
		`{{- "  "}}{{- color "cyan"}}[Use arrows to move, type to filter{{- if and .Help (not .ShowHelp)}}, {{ HelpInputRune }} for more help{{end}}]{{color "reset"}}`, ``, 1)
	// remove "? " prefix
	survey.MultiSelectQuestionTemplate = strings.Replace(survey.MultiSelectQuestionTemplate,
		`{{- color "green+hb"}}{{ QuestionIcon }} {{color "reset"}}`, "", 1)
	survey.MultiSelectQuestionTemplate = strings.Replace(survey.MultiSelectQuestionTemplate,
		`{{- if .ShowAnswer}}`,
		`{{- if not .ShowAnswer}}{{color "cyan"}} (use space to (multi)select, enter to confirm){{color "reset"}}{{end}}`+
			`{{- if .ShowAnswer}}`, 1)
	// "  " -> " " before option
	survey.MultiSelectQuestionTemplate = strings.Replace(survey.MultiSelectQuestionTemplate,
		`{{- " "}}{{$option}}`, "{{- $option}}", 1)
	survey.InputQuestionTemplate = strings.Replace(survey.InputQuestionTemplate,
		`{{- color "green+hb"}}{{ QuestionIcon }} {{color "reset"}}`, "", 1)
	survey.InputQuestionTemplate = strings.Replace(survey.InputQuestionTemplate,
		`[{{ HelpInputRune }} for help]`, "({{ .Help }})", 1)
	survey.InputQuestionTemplate = strings.Replace(survey.InputQuestionTemplate,
		`{{.Answer}}`, `{{or .Answer "\"\""}}`, 1)
	surveycore.MarkedOptionIcon = "+"
	surveycore.UnmarkedOptionIcon = " "
}

var newContext = nsxkubectl.NewContext

/*
var newContext = func () (nsx.Context, error) {
	return nsxkubectl.NewContextStub(func(user string, cluster string) ([]string, error) {
		if user == "minikube" && cluster == "minikube" {
			return []string{"default", "kube-public", "kube-system"}, nil
		}
		if user == "example@possibly-gmail.com" && cluster != "minikube" {
			return []string{"default", "kube-public", "kube-system", "dev", "testing", "staging"}, nil
		}
		return nil, fmt.Errorf("Unauthorized (%s:%s)", user, cluster)
	})
}
*/

func lazyContext() func() nsx.Context {
	var ctx nsx.Context
	return func() nsx.Context {
		if ctx == nil {
			var err error
			if ctx, err = newContext(); err != nil {
				log.Fatal(err)
			}
		}
		return ctx
	}
}

var validNS = regexp.MustCompile(`^[a-z0-9-.]+$`)
var whitespace = regexp.MustCompile("\\s+")

func main() {
	completion := cli.NewCompletion(lazyContext())
	completed, err := completion.Execute()
	if err != nil {
		log.Debug(err)
		os.Exit(3)
	}
	if completed {
		os.Exit(0)
	}
	rootCmd := &cobra.Command{
		Use:  "kubensx",
		Long: "Simpler Cluster/User/Namespace switching for Kubernetes (https://github.com/shyiko/kubensx).",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				log.SetLevel(log.DebugLevel)
			}
			if kubeconfig, _ := cmd.Flags().GetString("kubeconfig"); kubeconfig != "" {
				os.Setenv("KUBECONFIG", kubeconfig)
			}
			if noColor, _ := cmd.Flags().GetBool("no-color"); noColor {
				surveycore.DisableColor = true
				color.NoColor = true
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion, _ := cmd.Flags().GetBool("version"); showVersion {
				fmt.Println(version)
				return nil
			}
			return pflag.ErrHelp
		},
	}
	assocCmd := &cobra.Command{
		Use:     "assoc [pattern]",
		Aliases: []string{"a"},
		Short:   "Assoc[iate] user with one or more clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := newContext()
			if err != nil {
				log.Fatal(err)
			}
			dissociate, _ := cmd.Flags().GetBool("delete")
			if dissociate && len(args) == 0 {
				return errors.New("pattern (<user>:<cluster>) required")
			}
			dissociateAll, _ := cmd.Flags().GetBool("delete-all")
			if dissociateAll && len(args) != 0 {
				return errors.New("--delete-all and pattern cannot be used together")
			}
			if list, _ := cmd.Flags().GetBool("list"); list {
				if dissociate || dissociateAll {
					return errors.New("--list and --delete/--delete-all cannot be used together")
				}
				clustersByUser := ctx.ClustersByUser()
				var users []string
				for user := range clustersByUser {
					users = append(users, user)
				}
				for _, user := range sortInPlace(users) {
					for _, cluster := range sortInPlace(clustersByUser[user]) {
						fmt.Printf("%s:%s\n", user, cluster)
					}
				}
				return nil
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if len(args) == 0 && dryRun && !dissociateAll {
				return pflag.ErrHelp
			}
			if len(args) == 0 && !dryRun && !dissociateAll {
				mustContainAtLeastOneCluster(ctx)
				mustContainAtLeastOneUser(ctx)
				user := prompt("user:", sortInPlace(ctx.Users()), ctx.User(), true)
				defclusters := ctx.ClustersByUser()[user]
				clusters := promptMultiSelect("cluster:", ctx.Clusters(), defclusters)
			nextdef:
				for _, defcluster := range defclusters {
					for _, cluster := range clusters {
						if defcluster == cluster {
							continue nextdef
						}
					}
					ctx.Dissociate(user, defcluster)
					fmt.Printf("- %s:%s\n", user, defcluster)
				}
				for _, cluster := range clusters {
					if ctx.Associate(user, cluster) {
						fmt.Printf("+ %s:%s\n", user, cluster)
					}
				}
			} else {
				userMatcher := matchAll
				clusterMatcher := matchAll
				if len(args) != 0 {
					pattern := args[0]
					pattern, patternMatcher := newPatternMatcher(cmd, pattern)
					chunks := regexp.MustCompile(":").Split(pattern, 2)
					if chunks[0] == "" {
						log.Fatal("<user> cannot be empty")
					}
					userMatcher = bindMatcher(patternMatcher, chunks[0], ctx.User())
					if len(chunks) == 2 {
						// must be user:cluster (not just user)
						if chunks[1] == "" {
							log.Fatal("<cluster> cannot be empty")
						}
						clusterMatcher = bindMatcher(patternMatcher, chunks[1], ctx.Cluster())
					}
				}
				clusters := ctx.Clusters()
				assoc := ctx.ClustersByUser()
				clustersByUser := func(user string) []string { return clusters }
				if dissociate || dissociateAll {
					clustersByUser = func(user string) []string {
						return assoc[user]
					}
				}
				for _, user := range sortInPlace(userMatcher(ctx.Users())) {
					for _, cluster := range sortInPlace(clusterMatcher(clustersByUser(user))) {
						if dissociate || dissociateAll {
							if ctx.Dissociate(user, cluster) {
								fmt.Printf("- %s:%s\n", user, cluster)
							}
						} else {
							if ctx.Associate(user, cluster) {
								fmt.Printf("+ %s:%s\n", user, cluster)
							}
						}
					}
				}
			}
			if !dryRun {
				ctx.Commit()
			}
			return nil
		},
		Example: "  # assoc[iate] (interactive)\n" +
			"  kubensx assoc\n" +
			"  # assoc[iate] minikube user with minikube cluster\n" +
			"  kubensx assoc minikube:minikube\n" +
			"  \n" +
			"  # list assoc[iations]\n" +
			"  kubensx assoc -l\n" +
			"  \n" +
			"  # list <user>:<cluster> pairs that would be assoc[iated] should\n" +
			"  # `kubensx assoc <user>:<cluster>` be executed\n" +
			"  kubensx assoc --dry-run minikube\n" +
			"  kubensx assoc --dry-run '*:minikube'",
	}
	assocCmd.Flags().BoolP("delete", "d", false, "Delete assoc[iation](s)")
	assocCmd.Flags().Bool("delete-all", false, "Delete all assoc[iations]")
	assocCmd.Flags().BoolP("dry-run", "x", false, "Do not modify the config (just show what's going happen)")
	assocCmd.Flags().BoolP("exact", "e", false, "Match exactly (instead of default (wildcard) matching)")
	assocCmd.Flags().BoolP("fuzzy", "z", false, "Match fuzzily (instead of default (wildcard) matching)")
	assocCmd.Flags().BoolP("list", "l", false, "List assoc[iations] (<user>:<cluster>|s)")
	rootCmd.AddCommand(assocCmd)
	assocNsCmd := &cobra.Command{
		Use:     "config-ns [pattern...]",
		Aliases: []string{"n"},
		Short:   "Control list of namespaces",
		Long: "Control list of namespaces\n\n" +
			"Use config-ns when:" +
			"\n  - You wish to able to select a namespace from a list of options (e.g. \"kubensx use\")\nbut Access Control configuration prohibits user from listing namespaces; " +
			"\n  - You wish to reduce number of namespaces available for selection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := newContext()
			if err != nil {
				log.Fatal(err)
			}
			dissociate, _ := cmd.Flags().GetBool("delete")
			if dissociate && len(args) == 0 {
				return errors.New("pattern (<user>:<cluster>/<namespace>) required")
			}
			dissociateAll, _ := cmd.Flags().GetBool("delete-all")
			if dissociateAll && len(args) != 0 {
				return errors.New("--delete-all and pattern cannot be used together")
			}
			if list, _ := cmd.Flags().GetBool("list"); list {
				if dissociate || dissociateAll {
					return errors.New("--list and --delete/--delete-all cannot be used together")
				}
				for _, fqns := range sortFQNSSliceInPlace(ctx.ExplicitNamespaces()) {
					fmt.Printf("%s:%s/%s\n", fqns.User, fqns.Cluster, fqns.NS)
				}
				return nil
			}
			ignoreAssoc, _ := cmd.Flags().GetBool("ignore-assoc")
			if len(args) == 0 && !dissociateAll {
				mustContainAtLeastOneUser(ctx)
				mustContainAtLeastOneCluster(ctx)
				user := prompt("user:", sortInPlace(ctx.Users()), ctx.User(), true)
				clusters := ctx.ClustersByUser()[user]
				if ignoreAssoc || len(clusters) == 0 {
					clusters = ctx.Clusters()
				}
				cluster := prompt("cluster:", sortInPlace(clusters), ctx.Cluster(), true)
				var nss []string
				for _, r := range ctx.ExplicitNamespaces() {
					if r.User == user && r.Cluster == cluster {
						nss = append(nss, r.NS)
					}
				}
				sort.Strings(nss)
				input := promptInput("namespace(s):", strings.Join(nss, " "), "space-separated")
				var unss []string
				for _, m := range whitespace.Split(input, -1) {
					if m != "" {
						if err := validateNS(m); err != nil {
							log.Fatalf(err.Error())
						}
						unss = append(unss, m)
					}
				}
			nextNS:
				for _, ns := range nss {
					for _, uns := range unss {
						if ns == uns {
							continue nextNS
						}
					}
					ctx.DeleteExplicitNamespace(user, cluster, ns)
					fmt.Printf("- %s:%s/%s\n", user, cluster, ns)
				}
			nextUNS:
				for _, uns := range unss {
					for _, ns := range nss {
						if ns == uns {
							continue nextUNS
						}
					}
					ctx.SetExplicitNamespace(user, cluster, uns)
					fmt.Printf("+ %s:%s/%s\n", user, cluster, uns)
				}
			} else {
				var fqnss []nsx.FQNS
				if len(args) != 0 {
					for _, arg := range args {
						slashIndex := strings.LastIndex(arg, "/")
						if slashIndex == -1 {
							log.Fatalf(`Expected <user>:<cluster>/<namespace> or <cluster>/<namespace> (instead got "%s")`, arg)
						}
						namespace := arg[slashIndex+1:]
						if err := validateNS(namespace); err != nil {
							log.Fatal(err.Error())
						}
						pattern, patternMatcher := newPatternMatcher(cmd, arg[0:slashIndex])
						chunks := regexp.MustCompile(":").Split(pattern, 2)
						if len(chunks) == 1 {
							chunks = append([]string{"*"}, chunks...)
						}
						if chunks[0] == "" {
							log.Fatalf(`<user> cannot be empty ("%s")`, arg)
						}
						if chunks[1] == "" {
							log.Fatalf(`<cluster> cannot be empty ("%s")`, arg)
						}
						userMatcher := bindMatcher(patternMatcher, chunks[0], ctx.User())
						clusterMatcher := bindMatcher(patternMatcher, chunks[1], ctx.Cluster())
						clusters := ctx.Clusters()
						assoc := ctx.ClustersByUser()
						clustersByUser := func(user string) []string {
							r := assoc[user]
							if ignoreAssoc || len(r) == 0 {
								return clusters
							}
							return r
						}
						namespaceMatcher := bindMatcher(patternMatcher, namespace, ctx.Namespace())
						for _, ns := range namespaceMatcher([]string{namespace}) {
							for _, user := range userMatcher(ctx.Users()) {
								for _, cluster := range clusterMatcher(clustersByUser(user)) {
									fqnss = append(fqnss, nsx.FQNS{User: user, Cluster: cluster, NS: ns})
								}
							}
						}
					}
				} else {
					fqnss = ctx.ExplicitNamespaces()
				}
				for _, fqns := range sortFQNSSliceInPlace(fqnss) {
					if dissociate || dissociateAll {
						if ctx.DeleteExplicitNamespace(fqns.User, fqns.Cluster, fqns.NS) {
							fmt.Printf("- %s:%s/%s\n", fqns.User, fqns.Cluster, fqns.NS)
						}
					} else {
						if ctx.SetExplicitNamespace(fqns.User, fqns.Cluster, fqns.NS) {
							fmt.Printf("+ %s:%s/%s\n", fqns.User, fqns.Cluster, fqns.NS)
						}
					}
				}
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if !dryRun {
				ctx.Commit()
			}
			return nil
		},
		Example: "  # (interactive)\n" +
			"  kubensx config-ns\n" +
			"  # make staging namespace known for qa in us-west1 cluster\n" +
			"  kubensx config-ns qa:us-west1/staging\n" +
			"  \n" +
			"  # list assoc[iations]\n" +
			"  kubensx config-ns -l\n" +
			"  \n" +
			"  # list <user>:<cluster>/<namespace> triples that would be assoc[iated] should\n" +
			"  # `kubensx config-ns <user>:<cluster>/<namespace>` be executed\n" +
			"  kubensx config-ns --dry-run minikube/staging\n" +
			"  kubensx config-ns --dry-run '*:minikube/staging'",
	}
	assocNsCmd.Flags().BoolP("delete", "d", false, "Delete assoc[iation](s)")
	assocNsCmd.Flags().Bool("delete-all", false, "Delete all assoc[iations]")
	assocNsCmd.Flags().BoolP("dry-run", "x", false, "Do not modify the config (just show what's going happen)")
	assocNsCmd.Flags().BoolP("exact", "e", false, "Match exactly (instead of default (wildcard) matching)")
	assocNsCmd.Flags().BoolP("fuzzy", "z", false, "Match fuzzily (instead of default (wildcard) matching)")
	assocNsCmd.Flags().Bool("ignore-assoc", false, "Ignore user:cluster assoc[iations] (if any)")
	assocNsCmd.Flags().BoolP("list", "l", false, "List assoc[iations] (<user>:<cluster>/<namespace>|s)")
	rootCmd.AddCommand(assocNsCmd)
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: "Command-line completion",
	}
	completionCmd.AddCommand(
		&cobra.Command{
			Use:   "bash",
			Short: "Generate Bash completion",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) != 0 {
					return pflag.ErrHelp
				}
				if err := completion.GenBashCompletion(os.Stdout); err != nil {
					log.Error(err)
				}
				return nil
			},
			Example: "  source <(kubensx completion bash)",
		},
		&cobra.Command{
			Use:   "zsh",
			Short: "Generate Z shell completion",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) != 0 {
					return pflag.ErrHelp
				}
				if err := completion.GenZshCompletion(os.Stdout); err != nil {
					log.Error(err)
				}
				return nil
			},
			Example: "  source <(kubensx completion zsh)",
		},
	)
	rootCmd.AddCommand(completionCmd)
	currentCmd := &cobra.Command{
		Use:     "current",
		Aliases: []string{"c"},
		Short:   "Show current context (user:cluster/namespace)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := newContext()
			if err != nil {
				log.Fatal(err)
			}
			u, _ := cmd.Flags().GetBool("user")
			c, _ := cmd.Flags().GetBool("cluster")
			n, _ := cmd.Flags().GetBool("namespace")
			if !n {
				n, _ = cmd.Flags().GetBool("ns")
			}
			if u && !c && n {
				return errors.New("--cluster(-c) cannot be omitted when both --user(-u) and --namespace(--ns,-n) are present")
			}
			switch {
			case u == c == n:
				fmt.Println(formatContext(ctx))
			case u && c && !n:
				fmt.Printf("%s:%s\n", ctx.User(), ctx.Cluster())
			case !u && c && n:
				fmt.Printf("%s/%s\n", ctx.Cluster(), ctx.Namespace())
			case u:
				fmt.Println(ctx.User())
			case c:
				fmt.Println(ctx.Cluster())
			case n:
				fmt.Println(ctx.Namespace())
			}
			return nil
		},
	}
	currentCmd.Flags().BoolP("cluster", "c", false, "Output cluster only (can be combined with --user(-u) and --namespace(--ns,-n))")
	currentCmd.Flags().BoolP("namespace", "n", false, "Output namespace only (can be combined with --cluster(-c))")
	currentCmd.Flags().Bool("ns", false, "Alias for --namespace")
	currentCmd.Flags().BoolP("user", "u", false, "Output user only (can be combined with --cluster(-c))")
	rootCmd.AddCommand(currentCmd)
	lsCmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"l"},
		Short:   "List users/clusters/namespaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			u, _ := cmd.Flags().GetBool("users")
			c, _ := cmd.Flags().GetBool("clusters")
			n, _ := cmd.Flags().GetBool("namespaces")
			ignoreExplicitNS, _ := cmd.Flags().GetBool("ignore-config-ns")
			if !u && !c && !n {
				return pflag.ErrHelp
			}
			if u && c || u && n || c && n {
				return errors.New("--users(-u)/--clusters(-c)/--namespaces(-n) cannot be used together")
			}
			ctx, err := newContext()
			if err != nil {
				log.Fatal(err)
			}
			switch {
			case u:
				printWithSelectionHighlighted(ctx.Users(), ctx.User())
			case c:
				printWithSelectionHighlighted(ctx.Clusters(), ctx.Cluster())
			case n:
				printWithSelectionHighlighted(requireNamespaces(ctx, !ignoreExplicitNS), ctx.Namespace())
			}
			return nil
		},
	}
	lsCmd.Flags().BoolP("clusters", "c", false, "List clusters")
	lsCmd.Flags().BoolP("namespaces", "n", false, "List namespaces")
	lsCmd.Flags().BoolP("users", "u", false, "List users")
	lsCmd.Flags().Bool("ignore-config-ns", false, "Ignore explicit user:cluster/namespace(s) (if any)")
	rootCmd.AddCommand(lsCmd)
	useCmd := &cobra.Command{
		Use:     "use [user:cluster/namespace]",
		Aliases: []string{"u"},
		Short:   "Change context",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := newContext()
			if err != nil {
				log.Fatal(err)
			}
			u, _ := cmd.Flags().GetBool("user")
			c, _ := cmd.Flags().GetBool("cluster")
			n, _ := cmd.Flags().GetBool("namespace")
			if !n {
				n, _ = cmd.Flags().GetBool("ns")
			}
			if !u && !c && !n {
				u, c, n = true, true, true
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			ignoreAssoc, _ := cmd.Flags().GetBool("ignore-assoc")
			ignoreExplicitNS, _ := cmd.Flags().GetBool("ignore-config-ns")
			force, _ := cmd.Flags().GetBool("force")
			if len(args) == 0 {
				mustContainAtLeastOneCluster(ctx)
				mustContainAtLeastOneUser(ctx)
				ctx.SetCluster(prompt("cluster:", sortInPlace(ctx.Clusters()), ctx.Cluster(), c))
				users, user := sortInPlace(ctx.Users()), ctx.User()
				if !ignoreAssoc {
					assoc := ctx.UsersByCluster()[ctx.Cluster()]
					if len(assoc) != 0 {
						users = sortInPlace(assoc)
						if index(users, user) == -1 {
							user = users[0]
						}
					}
				}
				ctx.SetUser(prompt("user:", users, user, u))
				nss := requireNamespaces(ctx, !ignoreExplicitNS)
				if len(nss) == 0 {
					fmt.Println("\nIt appears that the user you have selected is not allowed to list namespaces.\n" +
						"If you wish to avoid manual entry next time you `kubensx use` - see `kubensx config-ns --help`.\n")
					ns := promptInput("namespace:", "", "")
					if err := validateNS(ns); err != nil {
						log.Fatalf(err.Error())
					}
					ctx.SetNamespace(ns)
				} else {
					ctx.SetNamespace(prompt("namespace:", sortInPlace(nss), ctx.Namespace(), n))
				}
			} else if args[0] == "-" {
				ctx.SetCluster(ctx.ClusterPrevious())
				ctx.SetUser(ctx.UserPrevious())
				ctx.SetNamespace(ctx.NamespacePrevious())
			} else {
				pattern := args[0]
				pattern, patternMatcher := newPatternMatcher(cmd, pattern)
				var user, cluster, namespace string
				var uexp, nexp bool // user/cluster/namespace explicit
				if !(u && c && n) {
					if u && c || u && n || c && n {
						return errors.New("--user(-u)/--cluster(-c)/--namespace(--ns,-n) cannot be used together")
					}
					user, cluster, namespace = ctx.User(), ctx.Cluster(), ctx.Namespace()
					switch {
					case u:
						uexp = true
						user = pattern
					case c:
						cluster = pattern
					case n:
						nexp = true
						namespace = pattern
					}
				} else {
					chunks := regexp.MustCompile("[:/]").Split(pattern, 3)
					switch len(chunks) {
					case 3: // user:cluster/namespace
						user, cluster, namespace = chunks[0], chunks[1], chunks[2]
						uexp, nexp = true, true
					case 2: // user:cluster or cluster/namespace
						if pattern[len(chunks[0])] == ':' {
							user, cluster, namespace = chunks[0], chunks[1], ctx.Namespace()
							uexp = true
						} else {
							user, cluster, namespace = ctx.User(), chunks[0], chunks[1]
							nexp = true
						}
					case 1: // namespace
						user, cluster, namespace = ctx.User(), ctx.Cluster(), chunks[0]
						nexp = true
					}
				}
				log.Debugf(`Searching for "%s(%v):%s/%s(%v)"`, user, uexp, cluster, namespace, nexp)
				// user and cluster can be empty per
				// https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/
				// (same goes for namespace)
				// have said that said, "" (empty) option should not be available for selection (through "prompt")
				clusterMatcher := stableMatcher(bindMatcher(patternMatcher, cluster, ctx.Cluster()))
				if cluster == "" {
					clusterMatcher = allowEmpty(clusterMatcher)
				}
				userMatcher := stableMatcher(bindMatcher(patternMatcher, user, ctx.User()))
				if user == "" {
					userMatcher = allowEmpty(userMatcher)
				}
				namespaceMatcher := stableMatcher(bindMatcher(patternMatcher, namespace, ctx.Namespace()))
				boundNSS := func() []string {
					if force {
						return []string{namespace}
					}
					r := requireNamespaces(ctx, !ignoreExplicitNS)
					if len(r) == 0 {
						log.Fatalf("It appears that \"%s\" is not allowed to list namespaces in \"%s\" cluster.\n"+
							"Either use --force(-f) (in which case namespace must be specified --exact|ly) or "+
							"provide an explicit list of namespaces via `kubensx config-ns`.", ctx.User(), ctx.Cluster())
					}
					return r
				}
				if namespace == "" {
					namespaceMatcher = allowEmpty(namespaceMatcher)
					boundNSS = func() []string { return []string{""} }
				}
				if !nexp {
					namespaceMatcher = fallbackToAllAvailable(namespaceMatcher)
				}
				assoc := ctx.UsersByCluster()
				usersByCluster := func(cluster string) []string {
					if ignoreAssoc {
						return ctx.Users()
					}
					users := assoc[cluster]
					if len(users) == 0 {
						users = ctx.Users()
					}
					return users
				}
				if !uexp {
					userMatcher = fallbackToAllAvailable(userMatcher)
				}
				if dryRun {
					for _, cluster := range clusterMatcher(ctx.Clusters()) {
						ctx.SetCluster(cluster)
						for _, user := range userMatcher(usersByCluster(cluster)) {
							ctx.SetUser(user)
							for _, namespace := range namespaceMatcher(boundNSS()) {
								ctx.SetNamespace(namespace)
								fmt.Println(formatContext(ctx))
							}
						}
					}
					return nil
				}
				mustContainAtLeastOneCluster(ctx)
				mustContainAtLeastOneUser(ctx)
				promptPattern := func(msg string, opts []string, def string, pattern string, matcher matcher) string {
					matches := matcher(opts)
					switch len(matches) {
					case 0:
						log.Fatalf(`"%s" does not match any of the %ss (expected one of (%s))`,
							pattern, msg, strings.Join(opts, ", "))
					case 1:
						return matches[0]
					}
					var opt = def
					if index(matches, def) == -1 {
						opt = matches[0]
					}
					match := prompt(msg+":", matches, opt, true)
					erasePreviousLine()
					return match
				}
				ctx.SetCluster(promptPattern("cluster", ctx.Clusters(), ctx.Cluster(), cluster, clusterMatcher))
				ctx.SetUser(promptPattern("user", usersByCluster(ctx.Cluster()), ctx.User(), user, userMatcher))
				ctx.SetNamespace(promptPattern("namespace", boundNSS(), ctx.Namespace(), namespace, namespaceMatcher))
			}
			if !dryRun {
				ctx.Commit()
			}
			fmt.Println("Switched to " + formatContext(ctx))
			return nil
		},
	}
	useCmd.Flags().BoolP("cluster", "c", false, "Change cluster only")
	useCmd.Flags().BoolP("dry-run", "x", false, "List matches (without changing the context)")
	useCmd.Flags().BoolP("exact", "e", false, "Match exactly (by default wildcard matching is used)")
	useCmd.Flags().BoolP("fuzzy", "z", false, "Match fuzzily (by default wildcard matching is used)")
	useCmd.Flags().Bool("ignore-assoc", false, "Ignore user:cluster assoc[iations] (if any)")
	useCmd.Flags().Bool("ignore-config-ns", false, "Ignore explicit user:cluster/namespace(s) (if any)")
	useCmd.Flags().BoolP("namespace", "n", false, "Change namespace only")
	useCmd.Flags().Bool("ns", false, "Alias for --namespace")
	useCmd.Flags().BoolP("user", "u", false, "Change user only")
	useCmd.Flags().BoolP("force", "f", false, "Skip namespace validation (NOTE: namespace must be provided --exact|ly)"+
		"\n(useful when user is not allowed to list namespaces; see also \"kubensx config-ns --help\")")
	rootCmd.AddCommand(useCmd)
	walk(rootCmd, func(cmd *cobra.Command) {
		cmd.Flags().BoolP("help", "h", false, "Print usage")
		cmd.Flags().MarkHidden("help")
	})
	rootCmd.PersistentFlags().Bool("debug", false, "Turn on debug output")
	rootCmd.PersistentFlags().String("kubeconfig", "", "Path to the config file (e.g. ~/.kube/config)")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
	rootCmd.Flags().Bool("version", false, "Print version information")
	if err := rootCmd.Execute(); err != nil {
		log.Debug(err)
		os.Exit(-1)
	}
}

func validateNS(ns string) error {
	if !validNS.MatchString(ns) {
		return fmt.Errorf(`"%s" is not a valid namespace`, ns)
	}
	return nil
}

func sortFQNSSliceInPlace(s []nsx.FQNS) []nsx.FQNS {
	sort.Slice(s, func(i, j int) bool {
		switch strings.Compare(s[i].User, s[j].User) {
		case -1:
			return true
		case 1:
			return false
		}
		switch strings.Compare(s[i].Cluster, s[j].Cluster) {
		case -1:
			return true
		case 1:
			return false
		}
		switch strings.Compare(s[i].NS, s[j].NS) {
		case -1:
			return true
		case 1:
			return false
		}
		return false
	})
	return s
}

func mustContainAtLeastOneCluster(ctx nsx.Context) {
	if len(ctx.Clusters()) == 0 {
		log.Fatal("No clusters have been found.\n" +
			"See `kubectl config set-cluster --help` on how to add one.")
	}
}

func mustContainAtLeastOneUser(ctx nsx.Context) {
	if len(ctx.Users()) == 0 {
		log.Fatal("No users have been found.\n" +
			"See `kubectl config set-credentials --help` on how to add one.")
	}
}

func requireNamespaces(ctx nsx.Context, explicit bool) []string {
	var r []string
	var err error
	if explicit {
		r, err = ctx.NamespaceView()
	} else {
		r, err = ctx.Namespaces()
	}
	if err != nil {
		log.Fatal(err)
	}
	return r
}

func formatContext(ctx nsx.Context) string {
	return fmt.Sprintf("%s:%s/%s", ctx.User(), ctx.Cluster(), ctx.Namespace())
}

type partialMatcher = func(pattern string, arr []string) []string
type matcher = func(arr []string) []string

func bindMatcher(m partialMatcher, pattern string, def string) matcher {
	switch pattern {
	case ".":
		return func(arr []string) []string { return []string{def} }
	case "*":
		return matchAll
	default:
		return func(arr []string) []string {
			return m(pattern, arr)
		}
	}
}

func fallbackToAllAvailable(m matcher) matcher {
	return func(arr []string) []string {
		r := m(arr)
		if len(r) == 0 {
			r = arr
		}
		return r
	}
}

func allowEmpty(m matcher) matcher {
	return func(arr []string) []string {
		return m(append([]string{""}, arr...))
	}
}

func stableMatcher(m matcher) matcher {
	return func(arr []string) []string {
		return sortInPlace(m(arr))
	}
}

func matchAll(arr []string) []string {
	return arr
}

func matchExact(v string, arr []string) []string {
	for _, a := range arr {
		if a == v {
			return []string{a}
		}
	}
	return nil
}

func matchWildcard(v string, arr []string) []string {
	var r []string
	split := strings.Split(v, "*")
nextstr:
	for _, str := range arr {
		if str == v {
			r = []string{v}
			break
		}
		for _, sub := range split {
			if sub != "" && !strings.Contains(str, sub) {
				continue nextstr
			}
		}
		r = append(r, str)
	}
	return r
}

func matchFuzzy(v string, arr []string) []string {
	if index(arr, v) != -1 {
		return []string{v}
	}
	return fuzzy.FindFold(v, arr)
}

func newPatternMatcher(cmd *cobra.Command, pattern string) (string, partialMatcher) {
	if exact, _ := cmd.Flags().GetBool("exact"); exact || strings.HasPrefix(pattern, "=") {
		return strings.TrimPrefix(pattern, "="), matchExact
	}
	if tilda, _ := cmd.Flags().GetBool("fuzzy"); tilda || strings.HasPrefix(pattern, "~") {
		return strings.TrimPrefix(pattern, "~"), matchFuzzy
	}
	return pattern, matchWildcard
}

func prompt(text string, opts []string, selection string, askUserToSelect bool) string {
	if askUserToSelect && len(opts) > 1 {
		return promptSelect(text, opts, selection)
	} else {
		if len(opts) == 1 {
			selection = opts[0]
		}
		return printSelect(text, selection)
	}
}

func printSelect(text string, value string) string {
	opt := value
	if opt == "" {
		opt = `""`
	}
	fmt.Println(text + " " + color.CyanString(opt))
	return value
}

func promptSelect(text string, opts []string, def string) string {
	value := def
	if err := survey.AskOne(
		&survey.Select{
			Message: text,
			Options: opts,
			Default: def,
		},
		&value,
		nil,
	); err != nil {
		log.Fatal(err)
	}
	return value
}

func promptMultiSelect(text string, opts []string, def []string) []string {
	value := def
	if err := survey.AskOne(
		&survey.MultiSelect{
			Message: text,
			Options: opts,
			Default: def,
		},
		&value,
		nil,
	); err != nil {
		log.Fatal(err)
	}
	return value
}

func promptInput(text string, def string, help string) string {
	value := def
	if err := survey.AskOne(
		&survey.Input{
			Message:     text,
			Default:     def,
			Help:        help,
		},
		&value,
		nil,
	); err != nil {
		log.Fatal(err)
	}
	return value
}

func erasePreviousLine() {
	surveyterminal.CursorPreviousLine(1)
	surveyterminal.EraseLine(surveyterminal.ERASE_LINE_ALL)
}

func printWithSelectionHighlighted(arr []string, selection string) {
	for _, namespace := range sortInPlace(arr) {
		if namespace == selection {
			namespace = color.CyanString(namespace)
		}
		fmt.Println(namespace)
	}
}

func index(arr []string, val string) int {
	for i, v := range arr {
		if v == val {
			return i
		}
	}
	return -1
}

func sortInPlace(arr []string) []string {
	sort.Strings(arr)
	return arr
}

func walk(cmd *cobra.Command, cb func(*cobra.Command)) {
	cb(cmd)
	for _, c := range cmd.Commands() {
		walk(c, cb)
	}
}
