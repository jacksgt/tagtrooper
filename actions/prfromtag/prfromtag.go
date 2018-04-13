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
	username    string
	apiToken    string
}

func New(url string, dir string, regex string, format string, filePattern string, username string, apiToken string) *Repository {
	var repo Repository

	repo.regexp = regexp.MustCompile(regex)
	repo.format = format
	repo.filePattern = regexp.MustCompile(filePattern)
	repo.dir = dir
	repo.username = username
	repo.apiToken = apiToken
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

	/* switch to it */
	d, err := os.Open(dir)
	if err != nil {
		fmt.Printf("%s\n", err)
		return nil
	}
	err = d.Chdir()
	d.Close()
	if err != nil {
		fmt.Printf("%s\n", err)
		return nil
	}

	/* initial clone */
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: url,
		//		Depth: 1, /* disable due to https://github.com/src-d/go-git/issues/802 */
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

func (repo *Repository) Run(tag string) (err error) {

	/* fetch updates */
	fmt.Print("Fetching origin... ")
	err = repo.r.Fetch(&git.FetchOptions{})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			fmt.Println("Repo already up to date")
		} else {
			fmt.Printf("failed: %s\n", err)
			return
		}
	} else {
		fmt.Println("ok")
	}

	branchName := fmt.Sprintf("tt-%s", tag)
	w, err := repo.r.Worktree()
	if err != nil {
		fmt.Printf("Failed to get worktree: %s", err)
		return
	}

	// headRef, err := repo.r.Head()
	// if err != nil {
	// 	fmt.Printf("%s\n", err)
	// 	return
	// }

	/* updated version */
	files := repo.updateVersionInAllFiles(tag)
	// err = repo.execUpdater(tag)
	if err != nil {
		fmt.Printf("Failed to execute: %s\n", err)
		return
	}

	for _, f := range files {
		// fmt.Println("Adding file", f)
		w.Add(f)
	}

	status, err := w.Status()
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	if status.IsClean() {
		fmt.Printf("Error: no files modified!\n")
		return
	}

	fmt.Print(status.String())

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

	/* create new branch */
	branchRef := plumbing.NewHashReference(
		plumbing.ReferenceName("refs/heads/"+branchName),
		commit,
	)
	err = repo.r.Storer.SetReference(branchRef)
	if err != nil {
		fmt.Printf("Failed to create branch %s on hash %s: %s\n", branchName, commit, err)
		return
	}

	/* create new tag */
	tagRef := plumbing.NewHashReference(
		plumbing.ReferenceName("refs/tags/"+tag),
		commit,
	)
	err = repo.r.Storer.SetReference(tagRef)
	if err != nil {
		fmt.Printf("Failed to create tag %s on hash %s: %s\n", tag, commit, err)
		return
	}

	/* push */
	auth := http.BasicAuth{
		Username: repo.username,
		Password: repo.apiToken,
	}

	branchRefName := "refs/heads/" + branchName
	tagRefName := "refs/tags/" + tag

	referenceList := append([]config.RefSpec{},
		config.RefSpec(plumbing.ReferenceName(branchRefName)+":"+plumbing.ReferenceName(branchRefName)),
		config.RefSpec(plumbing.ReferenceName(tagRefName)+":"+plumbing.ReferenceName(tagRefName)),
	)

	err = repo.r.Push(&git.PushOptions{
		RefSpecs: referenceList,
		Auth:     &auth,
	})
	if err != nil {
		fmt.Printf("Failed to push branch %s: %s\n", branchName, err)
		return
	}

	/* create PR */
	ts := &tokenSource{
		&oauth2.Token{AccessToken: repo.apiToken},
	}
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	msg := fmt.Sprintf("Update to %s\n -- Your loyal tag trooper", tag)

	pull := newPullRequest(
		"Update to "+tag,
		branchName,
		"master",
		msg,
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

func (repo *Repository) updateVersionInAllFiles(version string) (files []string) {
	newLine := fmt.Sprintf(repo.format, version)

	/* replace regex */
	callback := func(path string, fi os.FileInfo, err error) error {
		if fi.Name() == ".git" {
			return filepath.SkipDir
		}

		if fi.IsDir() {
			return nil
		}

		if !repo.filePattern.MatchString(fi.Name()) {
			return nil
		}

		fmt.Printf("Matching file: '%s'\n", fi.Name())

		err = replaceMatchingLinesInFile(path, repo.regexp, newLine)
		if err != nil {
			return nil
		}
		files = append(files, path)

		return nil
	}

	/* go through all files in repository */
	filepath.Walk(".", callback)

	return files
}

func replaceMatchingLinesInFile(path string, regexp *regexp.Regexp, newLine string) (err error) {
	lines, err := readLines(path)
	if err != nil {
		fmt.Printf("Failed to read from file %s: %s\n", path, err)
		return err
	}

	/* loop over all lines */
	for i := 0; i < len(lines); i++ {
		lines[i] = regexp.ReplaceAllString(lines[i], newLine)
		// fmt.Println(lines[i])
	}

	err = writeLines(lines, path)
	if err != nil {
		fmt.Printf("Failed to write to file %s: %s\n", path, err)
		return err
	}

	return nil
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
