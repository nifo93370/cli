package delete

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/prompt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type DeleteOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	RepoArg    string
	Confirmed  bool
}

func NewCmdDelete(f *cmdutil.Factory, runF func(*DeleteOptions) error) *cobra.Command {
	opts := &DeleteOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "delete <repository>",
		Short: "Delete a repository",
		Long: `Delete a GitHub repository.

Deletion requires authorization with the "delete_repo" scope. 
To authorize, run "gh auth refresh -s delete_repo"`,
		Args: cmdutil.ExactArgs(1, "cannot delete: repository argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RepoArg = args[0]
			if !opts.IO.CanPrompt() && !opts.Confirmed {
				return &cmdutil.FlagError{
					Err: errors.New("could not prompt: confirmation with prompt or --confirm flag required")}
			}
			if runF != nil {
				return runF(opts)
			}
			return deleteRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Confirmed, "confirm", false, "confirm deletion without prompting")
	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	httpClient, err := opts.HttpClient()
	if err != nil {
		return err
	}
	apiClient := api.NewClientFromHTTP(httpClient)

	repoSelector := opts.RepoArg
	var toDelete ghrepo.Interface

	if !strings.Contains(repoSelector, "/") {
		currentUser, err := api.CurrentLoginName(apiClient, ghinstance.Default())
		if err != nil {
			return err
		}
		repoSelector = currentUser + "/" + repoSelector
	}
	toDelete, err = ghrepo.FromFullName(repoSelector)
	if err != nil {
		return fmt.Errorf("argument error: %w", err)
	}

	fullName := ghrepo.FullName(toDelete)

	if !opts.Confirmed {
		var valid string
		err := prompt.SurveyAskOne(
			&survey.Input{Message: fmt.Sprintf("Type %s to confirm deletion:", fullName)},
			&valid,
			survey.WithValidator(
				func(val interface{}) error {
					if str := val.(string); !strings.EqualFold(str, fullName) {
						return fmt.Errorf("You entered %s", str)
					}
					return nil
				}))
		if err != nil {
			return fmt.Errorf("could not prompt: %w", err)
		}
	}

	err = deleteRepo(httpClient, toDelete)
	if err != nil {
		return err
	}

	if opts.IO.IsStdoutTTY() {
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out,
			"%s Deleted repository %s\n",
			cs.SuccessIcon(),
			fullName)
	}

	return nil
}
