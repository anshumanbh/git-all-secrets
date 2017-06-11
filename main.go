package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	uuid "github.com/satori/go.uuid"
)

// Info Function to show colored text
func Info(format string, args ...interface{}) {
	fmt.Printf("\x1b[34;1m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}

func check(e error) {
	if e != nil {
		panic(e)
	} else if _, ok := e.(*github.RateLimitError); ok {
		log.Println("hit rate limit")
	} else if _, ok := e.(*github.AcceptedError); ok {
		log.Println("scheduled on GitHub side")
	}
}

func gitclone(cloneURL string, repoName string, wg *sync.WaitGroup) {
	cmd := exec.Command("/usr/bin/git", "clone", cloneURL, repoName)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	check(err)
	wg.Done()
}

func cloneorgrepos(ctx context.Context, client *github.Client, org string) error {

	Info("Cloning the repositories of the organization: " + org)
	var orgRepos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}

	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
		check(err)
		orgRepos = append(orgRepos, repos...) //adding to the repo array
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var orgclone sync.WaitGroup
	//iterating through the repo array
	for _, repo := range orgRepos {
		orgclone.Add(1)
		fmt.Println(*repo.CloneURL)
		//cloning the individual org repos
		go gitclone(*repo.CloneURL, "/tmp/repos/org/"+*repo.Name, &orgclone)
	}
	orgclone.Wait()
	fmt.Println("")
	return nil
}

func cloneuserrepos(ctx context.Context, client *github.Client, user string) error {
	Info("Cloning " + user + "'s repositories")

	var userRepos []*github.Repository
	opt3 := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}

	for {
		uRepos, resp, err := client.Repositories.List(ctx, user, opt3)
		check(err)
		userRepos = append(userRepos, uRepos...) //adding to the userRepos array
		if resp.NextPage == 0 {
			break
		}
		opt3.Page = resp.NextPage
	}

	var userrepoclone sync.WaitGroup
	//iterating through the userRepos array
	for _, userRepo := range userRepos {
		userrepoclone.Add(1)
		fmt.Println(*userRepo.CloneURL)

		//cloning the individual user repos
		go gitclone(*userRepo.CloneURL, "/tmp/repos/users/"+user+"/"+*userRepo.Name, &userrepoclone)

	}

	userrepoclone.Wait()
	fmt.Println("")
	return nil
}

func cloneusergists(ctx context.Context, client *github.Client, user string) error {
	Info("Cloning " + user + "'s gists")

	var userGists []*github.Gist
	opt4 := &github.GistListOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}
	for {
		uGists, resp, err := client.Gists.List(ctx, user, opt4)
		check(err)
		userGists = append(userGists, uGists...)
		if resp.NextPage == 0 {
			break
		}
		opt4.Page = resp.NextPage
	}

	var usergistclone sync.WaitGroup
	//iterating through the userGists array
	for _, userGist := range userGists {
		usergistclone.Add(1)
		fmt.Println(*userGist.GitPullURL)

		//cloning the individual user gists
		go gitclone(*userGist.GitPullURL, "/tmp/repos/users/"+user+"/"+*userGist.ID, &usergistclone)
	}

	usergistclone.Wait()
	fmt.Println("")
	return nil
}

func listallusers(ctx context.Context, client *github.Client, org string) ([]*github.User, error) {
	Info("Users of the organization and their repositories and gists")
	var allUsers []*github.User
	opt2 := &github.ListMembersOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}

	for {
		users, resp, err := client.Organizations.ListMembers(ctx, org, opt2)
		check(err)
		allUsers = append(allUsers, users...) //adding to the allUsers array
		if resp.NextPage == 0 {
			break
		}
		opt2.Page = resp.NextPage
	}

	return allUsers, nil
}

func runGitTools(filepath string, wg *sync.WaitGroup, reponame string, orgoruser string) {
	outputFile1 := "/tmp/results/thog/" + orgoruser + "_" + reponame + "_" + uuid.NewV4().String() + ".txt"
	cmd1 := exec.Command("./thog/truffleHog/truffleHog.py", "-o", outputFile1, filepath)
	var out1 bytes.Buffer
	cmd1.Stdout = &out1
	err1 := cmd1.Run()
	check(err1)

	outputFile2 := "/tmp/results/gitsecrets/" + orgoruser + "_" + reponame + "_" + uuid.NewV4().String() + ".txt"
	cmd2 := exec.Command("./rungitsecrets.sh", filepath, outputFile2)
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	err2 := cmd2.Run()
	check(err2)

	wg.Done()
}

func scanforeachuser(user string, wg *sync.WaitGroup) {
	var wguserrepogist sync.WaitGroup
	gituserrepos, _ := ioutil.ReadDir("/tmp/repos/users/" + user)
	for _, f := range gituserrepos {
		wguserrepogist.Add(1)
		go runGitTools("/tmp/repos/users/"+user+"/"+f.Name()+"/", &wguserrepogist, f.Name(), user)
	}
	wguserrepogist.Wait()
	wg.Done()
}

func toolOutput(toolname string, of *os.File) error {

	linedelimiter := "----------------------------------------------------------------------------" +
		"----------------------------------------------------------------------------" +
		"----------------------------------------------------------------------------" +
		"----------------------------------------------------------------------------"

	_, err := of.WriteString("Tool: " + toolname + "\n")
	check(err)

	results, _ := ioutil.ReadDir("/tmp/results/" + toolname + "/")
	for _, f := range results {
		file, err := os.Open("/tmp/results/" + toolname + "/" + f.Name())
		check(err)

		fi, err := file.Stat()
		check(err)

		if fi.Size() == 0 {
			continue
		} else if fi.Size() > 0 {
			fname := strings.Split(f.Name(), "_")
			orgoruserstr := fname[0]
			rnamestr := fname[1]

			_, err1 := of.WriteString("OrgorUser: " + orgoruserstr + " RepoName: " + rnamestr + "\n")
			check(err1)

			if _, err2 := io.Copy(of, file); err2 != nil {
				return err2
			}

			_, err3 := of.WriteString(linedelimiter + "\n")
			check(err3)

			of.Sync()

		}
		defer file.Close()

	}

	return nil
}

func combineOutput(outputfile string) error {
	// Read all files in /tmp/results/<tool-name>/ directories for all the tools
	// open a new file and save it in the output directory - outputFile
	// for each results file, write user/org and reponame, copy results from the file in the outputFile, end with some delimiter

	of, err := os.Create(outputfile)
	check(err)

	tools := []string{"thog", "gitsecrets"}

	for _, tool := range tools {
		err = toolOutput(tool, of)
		check(err)
	}

	defer func() {
		os.RemoveAll("/tmp/repos/")
		os.RemoveAll("/tmp/results/")
		cerr := of.Close()
		if err == nil {
			err = cerr
		}
	}()

	return nil
}

func main() {

	org := flag.String("org", "", "Name of the Organization to scan")
	token := flag.String("token", "", "Github Personal Access Token")
	outputFile := flag.String("output", "", "Output file to save the results")
	flag.Parse()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	os.MkdirAll("/tmp/repos/org", 0700)
	os.MkdirAll("/tmp/repos/users", 0700)
	os.MkdirAll("/tmp/results/thog", 0700)
	os.MkdirAll("/tmp/results/gitsecrets", 0700)

	err := cloneorgrepos(ctx, client, *org)
	check(err)

	allUsers, err := listallusers(ctx, client, *org)
	check(err)

	//iterating through the allUsers array
	for _, user := range allUsers {

		err1 := cloneuserrepos(ctx, client, *user.Login)
		check(err1)

		err2 := cloneusergists(ctx, client, *user.Login)
		check(err2)

	}

	Info("We have all the repositories & gists now. Lets get to scanning them..\n")
	Info("Scanning all org repositories with truffleHog and git-secrets now..This may take a while so please be patient")

	var wgorg sync.WaitGroup

	gitorgrepos, _ := ioutil.ReadDir("/tmp/repos/org/")
	for _, f := range gitorgrepos {
		wgorg.Add(1)
		go runGitTools("/tmp/repos/org/"+f.Name()+"/", &wgorg, f.Name(), *org)
	}
	wgorg.Wait()

	Info("Finished scanning all org repositories\n")

	Info("Scanning all user repositories and gists now..This may take a while so please be patient")

	var wguser sync.WaitGroup
	for _, user := range allUsers {
		wguser.Add(1)
		go scanforeachuser(*user.Login, &wguser)
	}
	wguser.Wait()

	Info("Finished scanning all user repositories and gists\n")

	Info("Combining the output from all the tools into one file\n")
	err = combineOutput(*outputFile)
	check(err)

}
