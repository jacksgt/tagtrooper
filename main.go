package main

import (
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env"

	"github.com/jacksgt/tagtrooper/actions/prfromtag"
	"github.com/jacksgt/tagtrooper/provider/github"
)

type config struct {
	From           string `env:"TT_FROM"`
	To             string `env:"TT_TO"`
	Interval       int    `env:"TT_INTERVAL"`
	Once           bool   `env:"TT_ONCE"`
	GithubApiToken string `env:"TT_GITHUBAPITOKEN"`
	GithubUsername string `env:"TT_GITHUBUSERNAME"`
}

func main() {

	cfg := config{}
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%+v\n", cfg)

	/* choose provider */
	p := github.NewProvider(cfg.From, "tag")

	/* set up cron job */
	ticker := time.Tick(time.Duration(cfg.Interval) * time.Second)

	regex := `^ARG VERSION=[0-9]*\.[0-9]*\.[0-9]*.*`
	format := "ARG VERSION=%s\n"
	filePattern := `.*Dockerfile.*`
	const tmpPath string = "/tmp/tagtrooper/"

	/* set up destination repository */
	r := prfromtag.New(cfg.To, tmpPath, regex, format, filePattern, cfg.GithubUsername, cfg.GithubApiToken)
	if r == nil {
		fmt.Printf("Failed to set up repo %s\n", cfg.To)
		return
	}

	/* execute only once */
	if cfg.Once {
		tag := p.PrintLatestTag()
		if tag == "" {
			fmt.Printf("No tags found\n")
			os.Exit(0)
		}

		err = r.Run(tag)
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(1)
		}
		fmt.Printf("ok\n")
		os.Exit(0)
	}

	/* wait */
	for t := range ticker {
		/* check for update */
		if p.Check() {
			/* trigger action */
			fmt.Printf("New tag for repository %s (%s)", cfg.From, t)

			tag := p.PrintLatestTag()
			if tag == "" {
				fmt.Printf("Error: empty tag: %s\n", tag)
				continue
			}

			err = r.Run(tag)
			if err != nil {
				fmt.Printf("%s\n", err)
				continue
			}
			fmt.Printf("ok\n")
		}
	}
}
