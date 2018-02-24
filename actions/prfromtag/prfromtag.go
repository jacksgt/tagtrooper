package prfromtag

/* creates a github PR for a repository based on a tag */

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Repository struct {
	r           *git.Repository
	dir         string
	regexp      *regexp.Regexp
	format      string
	filePattern *regexp.Regexp
	owner       string
	name        string
}

func New(url string, dir string, regex string, format string, filePattern string) *Repository {
	var repo Repository

	repo.regexp = regexp.MustCompile(regex)
	repo.format = format
	repo.filePattern = regexp.MustCompile(filePattern)
	repo.dir = dir
	owner, name, err := splitUrl(url)
	if err != nil {
		return nil
	}
	repo.owner = owner
	repo.name = name

	/* create dir */
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		fmt.Printf("Failed to create dir %s: %s\n", dir, err)
		return nil
	}

	/* initial clone */
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:   url,
		Depth: 1,
	})
	if err == git.ErrRepositoryAlreadyExists {
		r, err = git.PlainOpen(dir)
		if err != nil {
			fmt.Printf("Failed to open repo %s in %s: %s\n", url, dir, err)
			return nil
		}
	} else if err != nil {
		fmt.Printf("Failed to clone %s into %s: %s\n", url, dir, err)
		return nil
	}

	repo.r = r

	return &repo
}

func (repo *Repository) Run(tag string) {
	githubApiToken := "xxx"
	githubUsername := "jacksgt"

	/* fetch updates */
	err := repo.r.Fetch(&git.FetchOptions{Depth: 1})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		fmt.Printf("Failed to fetch repo: %s\n", err)
		return
	}

	/* create new branch */
	branchName := tag
	w, err := repo.r.Worktree()
	if err != nil {
		fmt.Printf("Failed to get worktree: %s", err)
		return
	}

	headRef, err := repo.r.Head()
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}

	ref := plumbing.NewHashReference(
		plumbing.ReferenceName("refs/heads/"+branchName),
		headRef.Hash(),
	)

	err = repo.r.Storer.SetReference(ref)
	if err != nil {
		fmt.Printf("Failed to create branch %s on hash %s: %s\n", branchName, headRef.Hash(), err)
		return
	}

	/* updated version */
	// repo.updateVersionInAllDockerfiles(tag)
	err = repo.execUpdater(tag)
	if err != nil {
		fmt.Printf("Failed to execute: %s\n", err)
		return
	}

	/* add all changed files to staging area */
	w.Add("*")

	/* commit */
	commit, err := w.Commit("Update to tag "+tag, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Tag Trooper",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	if err != nil {
		fmt.Printf("Failed to commit: %s\n", err)
		return
	}

	/* create new tag */
	n := plumbing.ReferenceName("refs/tags/" + tag)
	t := plumbing.NewHashReference(n, commit)
	err = repo.r.Storer.SetReference(t)
	if err != nil {
		fmt.Printf("Failed to create tag %s on hash %s: %s\n", tag, commit, err)
		return
	}

	/* push branch */
	auth := http.BasicAuth{
		Username: githubUsername,
		Password: githubApiToken,
	}

	upstreamReference := plumbing.ReferenceName("refs/heads/" + branchName)
	downstreamReference := plumbing.ReferenceName("refs/heads/" + branchName)
	referenceList := append([]config.RefSpec{},
		config.RefSpec(upstreamReference+":"+downstreamReference),
	)

	err = repo.r.Push(&git.PushOptions{
		// RemoteName: branchName,
		RefSpecs: referenceList,
		Auth:     &auth,
	})
	if err != nil {
		fmt.Printf("Failed to push branch %s: %s\n", branchName, err)
		return
	}

	/* create PR */
	ts := &tokenSource{
		&oauth2.Token{AccessToken: githubApiToken},
	}
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	pull := newPullRequest(
		"Update to "+tag,
		branchName,
		"master",
		"Your loyal tag trooper",
	)

	pr, _, err := client.PullRequests.Create(context.Background(), repo.owner, repo.name, pull)
	if err != nil {
		fmt.Printf("Failed to create PR: %s\n", err)
		return
	}

	fmt.Printf("Created PR %d at %s\n", pr.GetNumber(), pr.GetURL())
	return

}

type tokenSource struct {
	token *oauth2.Token
}

func (t *tokenSource) Token() (*oauth2.Token, error) {
	return t.token, nil
}

func (repo *Repository) execUpdater(tag string) (err error) {
	cmd := exec.Command("./tagtrooper")
	cmd.Env = append(os.Environ(),
		"TT_TAG="+tag,
	)
	cmd.Dir = repo.dir
	err = cmd.Run()
	if err != nil {
		fmt.Printf("./tagtrooper: %s\n", err)
		return err
	}

	return nil
}

func (repo *Repository) updateVersionInAllDockerfiles(version string) {
	newLine := []byte(fmt.Sprintf(repo.format, version))

	/* replace "ARG [v|V]ersion[ ]=[ ][']*[']" with "ARG version='tag'" */
	callback := func(path string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}

		if !repo.filePattern.MatchString(path) {
			return nil
		}

		lines, err := readLines(path)
		if err != nil {
			fmt.Printf("Failed to read from file %s: %s\n", path, err)
			return err
		}

		/* loop over all lines */
		for _, l := range lines {
			repo.regexp.ReplaceAll([]byte(l), newLine)
		}

		err = writeLines(lines, path)
		if err != nil {
			fmt.Printf("Failed to write to file %s: %s\n", path, err)
			return err
		}
		return nil
	}

	/* go through all files in repository */
	filepath.Walk(".", callback)

}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

/* split [https://|http://|git://]github.com/owner/repo[.git]/... into owner, repo */
func splitUrl(url string) (owner string, repo string, err error) {
	str := url
	str = strings.TrimPrefix(str, "https://")
	str = strings.TrimPrefix(str, "http://")
	str = strings.TrimPrefix(str, "git://")

	if strings.HasPrefix(str, "github.com/") {
		str = strings.TrimPrefix(str, "github.com/")
	} else {
		err = errors.New("Wrong provider github for url " + url)
		return
	}

	fields := strings.Split(str, "/")
	if len(fields) < 2 {
		err = errors.New("Invalid format for provider github: " + url)
		return
	}

	owner = fields[0]
	repo = strings.TrimSuffix(fields[1], ".git")

	err = nil

	return
}

func newPullRequest(title, head, base, body string) *github.NewPullRequest {
	pull := &github.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
		Body:  &body,
	}
	return pull
}
