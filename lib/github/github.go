package github

import (
	"errors"
	"strings"
)

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
