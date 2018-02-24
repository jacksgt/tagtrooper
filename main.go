package main

import (
	"flag"
	"fmt"
	//	"time"

	"github.com/jacksgt/tagtrooper/actions/prfromtag"
	//	"github.com/jacksgt/tagtrooper/provider/github"
)

func main() {
	// /* fetch -url URL */
	// // wordPtr := flag.String("word", "foo", "a string")
	// fromurl := flag.String("fromurl", "https://github.com/jacksgt/test-1", "URL of monitor repository")
	// tourl := flag.String("tourl", "https://github.com/jacksgt/test-2", "URL of push repository")

	// // /* eval -trigger [commit,tag] */
	// // trigger := flag.String("trigger", "tag", "Trigger")
	// trigger := "tag"

	// /* fetch -interval */
	// interval := flag.Int("interval", 60, "Check interval in seconds")

	// regex := flag.String("regex", ".*[0-9]*.[0-9]*.[0-9]*.*", "Regex to replace")
	// format := flag.String("format", "%s\n", "format of the replacement")
	// filePattern := flag.String("replacefile", ".*Dockerfile.*", "pattern of files to replace in")

	// flag.Parse()

	// /* choose provider */
	// p := github.NewProvider(*fromurl, trigger)

	// /* set up cron job */
	// ticker := time.Tick(time.Duration(*interval) * time.Second)

	// /* set up repository */
	// r := prfromtag.New(*tourl, "/tmp/tagtrooper/", *regex, *format, *filePattern)
	// if r == nil {
	// 	fmt.Printf("Failed to set up repo %s\n", *tourl)
	// 	return
	// }

	// /* wait */
	// for t := range ticker {
	// 	/* check for update */
	// 	if p.Check() {
	// 		/* trigger action */
	// 		fmt.Printf("New %s for repository %s (%s)", trigger, *fromurl, t)
	// 		r.Run(p.PrintLatestTag())
	// 	}
	// }

	tourl := flag.String("tourl", "https://github.com/jacksgt/test-2", "URL of push repository")
	regex := flag.String("regex", ".*[0-9]*.[0-9]*.[0-9]*.*", "Regex to replace")
	format := flag.String("format", "%s\n", "format of the replacement")
	filePattern := flag.String("replacefile", ".*Dockerfile.*", "pattern of files to replace in")

	flag.Parse()

	r := prfromtag.New(*tourl, "/tmp/tagtrooper/", *regex, *format, *filePattern)
	if r == nil {
		fmt.Printf("Failed to set up repo %s\n", *tourl)
		return
	}

	r.Run("0.1.1")

}
