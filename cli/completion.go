package cli

import (
	"flag"
	"fmt"
	"github.com/posener/complete"
	nsx "github.com/shyiko/kubensx/context"
	"io"
	"os"
	"path/filepath"
)

type Completion struct {
	ctx func() nsx.Context
}

func (c *Completion) GenBashCompletion(w io.Writer) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "complete -C %s %s\n", bin, filepath.Base(bin))
	return nil
}

func (c *Completion) GenZshCompletion(w io.Writer) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "autoload +X compinit && compinit\nautoload +X bashcompinit && bashcompinit\ncomplete -C %s %s\n",
		bin, filepath.Base(bin))
	return nil
}

// complete.PredictSet(...) alternative
type oneOf []string

func (p oneOf) Predict(args complete.Args) []string {
	for _, opt := range p {
		if args.LastCompleted == opt {
			return nil
		}
	}
	return p
}

func (c *Completion) Execute() (bool, error) {
	bin, err := os.Executable()
	if err != nil {
		return false, err
	}
	run := complete.Command{
		Sub: complete.Commands{
			"assoc": complete.Command{
				Flags: complete.Flags{
					"--delete":     complete.PredictNothing,
					"-d":           complete.PredictNothing,
					"--delete-all": complete.PredictNothing,
					"--dry-run":    complete.PredictNothing,
					"-x":           complete.PredictNothing,
					"--exact":      complete.PredictNothing,
					"-e":           complete.PredictNothing,
					"--fuzzy":      complete.PredictNothing,
					"-z":           complete.PredictNothing,
					"--list":       complete.PredictNothing,
					"-l":           complete.PredictNothing,
				},
				// todo:
				// initially show user: & user:*, once completed user:...
				// Args: oneOf(c.ctx().Users()),
			},
			"completion": complete.Command{
				Sub: complete.Commands{
					"bash": complete.Command{},
					"zsh":  complete.Command{},
				},
			},
			"current": complete.Command{
				Flags: complete.Flags{
					"--cluster":   complete.PredictNothing,
					"-c":          complete.PredictNothing,
					"--namespace": complete.PredictNothing,
					"--ns":        complete.PredictNothing,
					"-n":          complete.PredictNothing,
					"--user":      complete.PredictNothing,
					"-u":          complete.PredictNothing,
				},
			},
			"ls": complete.Command{
				Flags: complete.Flags{
					"--users":      complete.PredictNothing,
					"-u":           complete.PredictNothing,
					"--clusters":   complete.PredictNothing,
					"-c":           complete.PredictNothing,
					"--namespaces": complete.PredictNothing,
					"-n":           complete.PredictNothing,
				},
			},
			"use": complete.Command{
				Flags: complete.Flags{
					"--cluster":      complete.PredictNothing,
					"-c":             complete.PredictNothing,
					"--dry-run":      complete.PredictNothing,
					"-x":             complete.PredictNothing,
					"--exact":        complete.PredictNothing,
					"-e":             complete.PredictNothing,
					"--fuzzy":        complete.PredictNothing,
					"-z":             complete.PredictNothing,
					"--ignore-assoc": complete.PredictNothing,
					"--namespace":    complete.PredictNothing,
					"--ns":           complete.PredictNothing,
					"-n":             complete.PredictNothing,
					"--user":         complete.PredictNothing,
					"-u":             complete.PredictNothing,
				},
				// todo:
				// Args: oneOf(c.ctx().Users()),
			},
			"help": complete.Command{
				Sub: complete.Commands{
					"assoc": complete.Command{},
					"completion": complete.Command{
						Sub: complete.Commands{
							"bash": complete.Command{},
							"zsh":  complete.Command{},
						},
					},
					"current": complete.Command{},
					"ls":      complete.Command{},
					"use":     complete.Command{},
				},
			},
		},
		GlobalFlags: complete.Flags{
			"--kubeconfig": complete.PredictFiles("*"),
			"--debug":      complete.PredictNothing,
			"--version":    complete.PredictNothing,
			"--help":       complete.PredictNothing,
		},
	}
	completion := complete.New(filepath.Base(bin), run)
	if os.Getenv("COMP_LINE") != "" {
		flag.Parse()
		completion.Complete()
		return true, nil
	}
	return false, nil
}

func NewCompletion(ctx func() nsx.Context) *Completion {
	return &Completion{ctx}
}
