package github

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/github"
	"strings"
)

type GithubProvider struct {
	client  *github.Client
	trigger string
	owner   string
	repo    string
	lastTag *github.RepositoryTag
}

func NewProvider(url string, trigger string) (p GithubProvider) {
	var err error
	p.trigger = trigger
	p.owner, p.repo, err = splitUrl(url)
	if err != nil {
		fmt.Println("Invalid format: ", url)
		return
	}

	/* create new github client */
	p.client = github.NewClient(nil)

	if p.trigger == "tag" {
		p.lastTag = p.getLatestTag()
		if p.lastTag == nil {
			fmt.Println("No tag yet")
		} else {
			fmt.Println("Last known tag: ", *p.lastTag.Name, *p.lastTag.Commit.SHA)
		}
	} else {
		fmt.Println("Not implemented.")
		return
	}

	return p

}

func (p *GithubProvider) Check() bool {
	if p.trigger == "tag" {
		return p.checkTag()
	}

	fmt.Println("Not implemented")
	return false
}

func (p *GithubProvider) checkTag() bool {
	current := p.getLatestTag()
	if current == nil {
		fmt.Println("No tags yet")
		return false
	}

	if *current.Commit.SHA != *p.lastTag.Commit.SHA {
		fmt.Println("New tag: ", *current.Name, *current.Commit.SHA)
		p.lastTag = current
		return true
	}

	fmt.Println("No new tag")
	return false
}

func (p *GithubProvider) getLatestTag() (tag *github.RepositoryTag) {
	/* fetch all tags */
	tags, _, err := p.client.Repositories.ListTags(context.Background(), p.owner, p.repo, nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if len(tags) >= 1 {
		return tags[0]
	}

	return nil
}

func (p *GithubProvider) PrintLatestTag() string {
	if p.lastTag == nil {
		return ""
	}
	return *p.lastTag.Name
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
