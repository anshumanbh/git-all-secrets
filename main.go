package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
)

var (
	org                  = flag.String("org", "", "Name of the Organization to scan. Example: secretorg123")
	token                = flag.String("token", "", "Github Personal Access Token. This is required.")
	outputFile           = flag.String("output", "results.txt", "Output file to save the results.")
	user                 = flag.String("user", "", "Name of the Github user to scan. Example: secretuser1")
	repoURL              = flag.String("repoURL", "", "HTTPS URL of the Github repo to scan. Example: https://github.com/anshumantestorg/repo1.git")
	gistURL              = flag.String("gistURL", "", "HTTPS URL of the Github gist to scan. Example: https://gist.github.com/secretuser1/81963f276280d484767f9be895316afc")
	cloneForks           = flag.Bool("cloneForks", false, "Option to clone org and user repos that are forks. Default is false")
	orgOnly              = flag.Bool("orgOnly", false, "Option to skip cloning user repo's when scanning an org. Default is false")
	toolName             = flag.String("toolName", "all", "Specify whether to run thog or repo-supervisor")
	teamName             = flag.String("teamName", "", "Name of the Organization Team which has access to private repositories for scanning.")
	scanPrivateReposOnly = flag.Bool("scanPrivateReposOnly", false, "Option to scan private repositories only. Default is false")
	enterpriseURL        = flag.String("enterpriseURL", "", "Base URL of the Github Enterprise")
	threads              = flag.Int("threads", 10, "Amount of parallel threads")
	thogEntropy          = flag.Bool("thogEntropy", false, "Option to include high entropy secrets when truffleHog is used")
	mergeOutput          = flag.Bool("mergeOutput", false, "Merge the output files of all the tools used into one JSON file")
	blacklist            = flag.String("blacklist", "", "Comma seperated values of Repos to Skip Scanning for")
	executionQueue       chan bool
)

type truffleHogOutput struct {
	Branch       string   `json:"branch"`
	Commit       string   `json:"commit"`
	CommitHash   string   `json:"commitHash"`
	Date         string   `json:"date"`
	Diff         string   `json:"diff"`
	Path         string   `json:"path"`
	PrintDiff    string   `json:"printDiff"`
	Reason       string   `json:"reason"`
	StringsFound []string `json:"stringsFound"`
}

type reposupervisorOutput struct {
	Result map[string][]string `json:"result"`
}

type repositoryScan struct {
	Repository string              `json:"repository"`
	Results    map[string][]string `json:"stringsFound"`
}

func enqueueJob(item func()) {
	executionQueue <- true
	go func() {
		item()
		<-executionQueue
	}()
}

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
	defer wg.Done()

	cmd := exec.Command("/usr/bin/git", "clone", cloneURL, repoName)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		// panic(err)
	}
}

func gitRepoURL(path string) (string, error) {
	out, err := exec.Command("/usr/bin/git", "-C", path, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return "", err
	}
	url := strings.TrimSuffix(string(out), "\n")
	return url, nil
}

// Moving cloning logic out of individual functions
func executeclone(repo *github.Repository, directory string, wg *sync.WaitGroup) {
	urlToClone := ""

	switch *scanPrivateReposOnly {
	case false:
		urlToClone = *repo.CloneURL
	case true:
		urlToClone = *repo.SSHURL
	default:
		urlToClone = *repo.CloneURL
	}

	if *enterpriseURL != "" {
		urlToClone = *repo.SSHURL
	}

	var orgclone sync.WaitGroup
	if !*cloneForks && *repo.Fork {
		fmt.Println(*repo.Name + " is a fork and the cloneFork flag was set to false so moving on..")
	} else {
		// clone it
		orgclone.Add(1)
		fmt.Println(urlToClone)
		func(orgclone *sync.WaitGroup, urlToClone string, directory string) {
			enqueueJob(func() {
				gitclone(urlToClone, directory, orgclone)
			})
		}(&orgclone, urlToClone, directory)
	}

	orgclone.Wait()
	wg.Done()
}

func cloneorgrepos(ctx context.Context, client *github.Client, org string) error {

	Info("Cloning the repositories of the organization: " + org)
	Info("If the token provided belongs to a user in this organization, this will also clone all public AND private repositories of this org, irrespecitve of the scanPrivateReposOnly flag being set..")

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

	var orgrepowg sync.WaitGroup

	//iterating through the repo array
	for _, repo := range orgRepos {
		if strings.Contains(*blacklist, *repo.Name) {
			fmt.Println("Repo " + *repo.Name + " is in the repo blacklist, moving on..")
		} else {
			orgrepowg.Add(1)
			go executeclone(repo, "/tmp/repos/org/"+org+"/"+*repo.Name, &orgrepowg)
		}
	}

	orgrepowg.Wait()
	fmt.Println("Done cloning org repos.")
	return nil
}

func cloneuserrepos(ctx context.Context, client *github.Client, user string) error {
	Info("Cloning " + user + "'s repositories")
	Info("If the scanPrivateReposOnly flag is set, this will only scan the private repositories of this user. If that flag is not set, only public repositories are scanned. ")

	var uname string
	var userRepos []*github.Repository
	var opt3 *github.RepositoryListOptions

	if *scanPrivateReposOnly {
		uname = ""
		opt3 = &github.RepositoryListOptions{
			Visibility:  "private",
			ListOptions: github.ListOptions{PerPage: 10},
		}
	} else {
		uname = user
		opt3 = &github.RepositoryListOptions{
			ListOptions: github.ListOptions{PerPage: 10},
		}
	}

	for {
		uRepos, resp, err := client.Repositories.List(ctx, uname, opt3)
		check(err)
		userRepos = append(userRepos, uRepos...) //adding to the userRepos array
		if resp.NextPage == 0 {
			break
		}
		opt3.Page = resp.NextPage
	}

	var userrepowg sync.WaitGroup
	//iterating through the userRepos array
	for _, userRepo := range userRepos {
		userrepowg.Add(1)
		go executeclone(userRepo, "/tmp/repos/users/"+user+"/"+*userRepo.Name, &userrepowg)
	}

	userrepowg.Wait()
	fmt.Println("Done cloning user repos.")
	return nil
}

func cloneusergists(ctx context.Context, client *github.Client, user string) error {
	Info("Cloning " + user + "'s gists")
	Info("Irrespective of the scanPrivateReposOnly flag being set or not, this will scan all public AND secret gists of a user whose token is provided")

	var gisturl string

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

		if *enterpriseURL != "" {
			d := strings.Split(*userGist.GitPullURL, "/")[2]
			f := strings.Split(*userGist.GitPullURL, "/")[4]
			gisturl = "git@" + d + ":gist/" + f
		} else {
			gisturl = *userGist.GitPullURL
		}

		fmt.Println(gisturl)

		//cloning the individual user gists
		func(gisturl string, userGist *github.Gist, user string, usergistclone *sync.WaitGroup) {
			enqueueJob(func() {
				gitclone(gisturl, "/tmp/repos/users/"+user+"/"+*userGist.ID, usergistclone)
			})
		}(gisturl, userGist, user, &usergistclone)
	}

	usergistclone.Wait()
	return nil
}

func listallusers(ctx context.Context, client *github.Client, org string) ([]*github.User, error) {
	Info("Listing users of the organization and their repositories and gists")
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

func runTrufflehog(filepath string, reponame string, orgoruser string) error {
	outputDir := "/tmp/results/" + orgoruser + "/" + reponame
	os.MkdirAll(outputDir, 0700)
	outputFile1 := outputDir + "/" + "truffleHog"

	// open the out file for writing
	outfile, fileErr := os.OpenFile(outputFile1, os.O_CREATE|os.O_RDWR, 0644)
	check(fileErr)
	defer outfile.Close()

	params := []string{filepath, "--rules=/root/truffleHog/rules.json", "--regex"}
	if *mergeOutput {
		params = append(params, "--json")
	}
	var cmd1 *exec.Cmd

	if *thogEntropy {
		params = append(params, "--entropy=True")
	} else {
		params = append(params, "--entropy=False")
	}
	cmd1 = exec.Command("trufflehog", params...)

	// direct stdout to the outfile
	cmd1.Stdout = outfile

	err1 := cmd1.Run()
	// truffleHog returns an exit code 1 if it finds anything
	if err1 != nil && err1.Error() != "exit status 1" {
		Info("truffleHog Scanning failed for: " + orgoruser + "_" + reponame + ". Please scan it manually.")
		fmt.Println(err1)
	} else {
		fmt.Println("Finished truffleHog Scanning for: " + orgoruser + "_" + reponame)
	}

	return nil
}

func runReposupervisor(filepath string, reponame string, orgoruser string) error {
	outputDir := "/tmp/results/" + orgoruser + "/" + reponame
	os.MkdirAll(outputDir, 0700)
	outputFile3 := outputDir + "/" + "repo-supervisor"

	cmd3 := exec.Command("/root/repo-supervisor/runreposupervisor.sh", filepath, outputFile3)
	var out3 bytes.Buffer
	cmd3.Stdout = &out3
	err3 := cmd3.Run()
	if err3 != nil {
		Info("Repo Supervisor Scanning failed for: " + orgoruser + "_" + reponame + ". Please scan it manually.")
		fmt.Println(err3)
	} else {
		fmt.Println("Finished Repo Supervisor Scanning for: " + orgoruser + "_" + reponame)
	}
	return nil
}

func runGitTools(tool string, filepath string, wg *sync.WaitGroup, reponame string, orgoruser string) {
	defer wg.Done()

	switch tool {
	case "all":
		err := runTrufflehog(filepath, reponame, orgoruser)
		check(err)
		err = runReposupervisor(filepath, reponame, orgoruser)
		check(err)

	case "thog":
		err := runTrufflehog(filepath, reponame, orgoruser)
		check(err)

	case "repo-supervisor":
		err := runReposupervisor(filepath, reponame, orgoruser)
		check(err)
	}
}

func scanforeachuser(user string, wg *sync.WaitGroup) {
	defer wg.Done()

	var wguserrepogist sync.WaitGroup
	gituserrepos, _ := ioutil.ReadDir("/tmp/repos/users/" + user)
	for _, f := range gituserrepos {
		wguserrepogist.Add(1)
		func(user string, wg *sync.WaitGroup, wguserrepogist *sync.WaitGroup, f os.FileInfo) {
			enqueueJob(func() {
				runGitTools(*toolName, "/tmp/repos/users/"+user+"/"+f.Name()+"/", wguserrepogist, f.Name(), user)
			})
		}(user, wg, &wguserrepogist, f)
	}
	wguserrepogist.Wait()
}

func toolsOutput(toolname string, of *os.File) error {

	linedelimiter := "----------------------------------------------------------------------------" +
		"----------------------------------------------------------------------------" +
		"----------------------------------------------------------------------------" +
		"----------------------------------------------------------------------------"

	_, err := of.WriteString("Tool: " + toolname + "\n")
	check(err)

	users, _ := ioutil.ReadDir("/tmp/results/")
	for _, user := range users {
		repos, _ := ioutil.ReadDir("/tmp/results/" + user.Name() + "/")
		for _, repo := range repos {
			file, err := os.Open("/tmp/results/" + user.Name() + "/" + repo.Name() + "/" + toolname)
			check(err)

			fi, err := file.Stat()
			check(err)

			if fi.Size() == 0 {
				continue
			} else if fi.Size() > 0 {
				orgoruserstr := user.Name()
				rnamestr := repo.Name()

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
	}
	return nil
}

func singletoolOutput(toolname string, of *os.File) error {

	users, _ := ioutil.ReadDir("/tmp/results/")
	for _, user := range users {
		repos, _ := ioutil.ReadDir("/tmp/results/" + user.Name() + "/")
		for _, repo := range repos {
			file, err := os.Open("/tmp/results/" + user.Name() + "/" + repo.Name() + "/" + toolname)
			check(err)

			fi, err := file.Stat()
			check(err)

			if fi.Size() == 0 {
				continue
			} else if fi.Size() > 0 {

				if _, err2 := io.Copy(of, file); err2 != nil {
					return err2
				}
				of.Sync()
			}
			defer file.Close()
		}
	}
	return nil
}

func combineOutput(toolname string, outputfile string) error {
	// Read all files in /tmp/results/<tool-name>/ directories for all the tools
	// open a new file and save it in the output directory - outputFile
	// for each results file, write user/org and reponame, copy results from the file in the outputFile, end with some delimiter

	of, err := os.Create(outputfile)
	check(err)

	switch toolname {
	case "all":
		tools := []string{"truffleHog", "repo-supervisor"}

		for _, tool := range tools {
			err = toolsOutput(tool, of)
			check(err)
		}
	case "truffleHog":
		err = singletoolOutput("truffleHog", of)
		check(err)
	case "repo-supervisor":
		err = singletoolOutput("repo-supervisor", of)
		check(err)
	}

	defer func() {
		cerr := of.Close()
		if err == nil {
			err = cerr
		}
	}()

	return nil
}

func mergeOutputJSON(outputfile string) {
	var results []repositoryScan
	var basePaths []string

	if *repoURL != "" || *gistURL != "" {
		basePaths = []string{"/tmp/repos"}
	} else {
		basePaths = []string{"/tmp/repos/org", "/tmp/repos/users", "/tmp/repos/team"}
	}

	for _, basePath := range basePaths {
		users, _ := ioutil.ReadDir(basePath)
		for _, user := range users {
			repos, _ := ioutil.ReadDir("/tmp/results/" + user.Name() + "/")
			for _, repo := range repos {
				repoPath := basePath + "/" + user.Name() + "/" + repo.Name() + "/"
				repoResultsPath := "/tmp/results/" + user.Name() + "/" + repo.Name() + "/"
				reposupvPath := repoResultsPath + "repo-supervisor"
				thogPath := repoResultsPath + "truffleHog"
				reposupvExists := fileExists(reposupvPath)
				thogExists := fileExists(thogPath)
				repoURL, _ := gitRepoURL(repoPath)

				var mergedOut map[string][]string
				if reposupvExists && thogExists {
					reposupvOut, _ := loadReposupvOut(reposupvPath, repoPath)
					thogOut, _ := loadThogOutput(thogPath)
					mergedOut = mergeOutputs(reposupvOut, thogOut)
				} else if reposupvExists {
					mergedOut, _ = loadReposupvOut(reposupvPath, repoPath)
				} else if thogExists {
					mergedOut, _ = loadThogOutput(thogPath)
				}
				if len(mergedOut) > 0 {
					results = append(results, repositoryScan{Repository: repoURL, Results: mergedOut})
				}
			}
		}
	}
	marshalledResults, err := json.Marshal(results)
	check(err)
	err = ioutil.WriteFile(outputfile, marshalledResults, 0644)
	check(err)
}

func appendIfMissing(slice []string, i string) []string {
	for _, ele := range slice {
		if ele == i {
			return slice
		}
	}
	return append(slice, i)
}

func loadThogOutput(outfile string) (map[string][]string, error) {
	results := make(map[string][]string)
	output, err := ioutil.ReadFile(outfile)
	if err != nil {
		return nil, err
	}

	// There was an issue concerning truffleHog's output not being valid JSON
	// https://github.com/dxa4481/truffleHog/issues/95
	// but apparently it was closed without a fix.
	entries := strings.Split(string(output), "\n")
	for _, entry := range entries[:len(entries)-1] {
		var issue truffleHogOutput
		err := json.Unmarshal([]byte(entry), &issue)
		if err != nil {
			return nil, err
		}
		if _, found := results[issue.Path]; found {
			for _, str := range issue.StringsFound {
				results[issue.Path] = appendIfMissing(results[issue.Path], str)
			}
		} else {
			results[issue.Path] = issue.StringsFound
		}

	}
	return results, nil
}

func loadReposupvOut(outfile string, home string) (map[string][]string, error) {
	results := make(map[string][]string)
	output, err := ioutil.ReadFile(outfile)
	if err != nil {
		return nil, err
	}

	var rsupervisorOutput reposupervisorOutput
	json.Unmarshal(output, &rsupervisorOutput)
	for path, stringFound := range rsupervisorOutput.Result {
		relativePath := strings.TrimPrefix(path, home)
		// Make sure there aren't any leading slashes
		fileName := strings.TrimPrefix(relativePath, "/")
		results[fileName] = stringFound
	}

	return results, nil
}

func mergeOutputs(outputA map[string][]string, outputB map[string][]string) map[string][]string {
	for path, stringsFound := range outputA {
		if _, included := outputB[path]; included {
			outputB[path] = append(outputB[path], stringsFound...)
		} else {
			outputB[path] = stringsFound
		}
	}

	return outputB
}

// Moving directory scanning logic out of individual functions
func scanDir(dir string, org string) error {
	var wg sync.WaitGroup

	allRepos, _ := ioutil.ReadDir(dir)
	for _, f := range allRepos {
		wg.Add(1)
		func(f os.FileInfo, wg *sync.WaitGroup, org string) {
			enqueueJob(func() {
				runGitTools(*toolName, dir+f.Name()+"/", wg, f.Name(), org)
			})
		}(f, &wg, org)

	}
	wg.Wait()
	return nil
}

func scanorgrepos(org string) error {
	err := scanDir("/tmp/repos/org/"+org+"/", org)
	check(err)
	return nil
}

func stringInSlice(a string, list []*github.Repository) (bool, error) {
	for _, b := range list {
		if *b.SSHURL == a || *b.CloneURL == a {
			return true, nil
		}
	}
	return false, nil
}

func checkifsshkeyexists() error {
	fmt.Println("Checking to see if the SSH key exists or not..")

	fi, err := os.Stat("/root/.ssh/id_rsa")
	if err == nil && fi.Size() > 0 {
		fmt.Println("SSH key exists and file size > 0 so continuing..")
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	return nil
}

func checkflags(token string, org string, user string, repoURL string, gistURL string, teamName string, scanPrivateReposOnly bool, orgOnly bool, toolName string, enterpriseURL string, thogEntropy bool) error {
	if token == "" {
		fmt.Println("Need a Github personal access token. Please provide that using the -token flag")
		os.Exit(2)
	} else if org == "" && user == "" && repoURL == "" && gistURL == "" {
		fmt.Println("org, user, repoURL and gistURL can't all be empty. Please provide just one of these values")
		os.Exit(2)
	} else if org != "" && (user != "" || repoURL != "" || gistURL != "") {
		fmt.Println("Can't have org along with any of user, repoURL or gistURL. Please provide just one of these values")
		os.Exit(2)
	} else if user != "" && (org != "" || repoURL != "" || gistURL != "") {
		fmt.Println("Can't have user along with any of org, repoURL or gistURL. Please provide just one of these values")
		os.Exit(2)
	} else if repoURL != "" && (org != "" || user != "" || gistURL != "") {
		fmt.Println("Can't have repoURL along with any of org, user or gistURL. Please provide just one of these values")
		os.Exit(2)
	} else if gistURL != "" && (org != "" || repoURL != "" || user != "") {
		fmt.Println("Can't have gistURL along with any of org, user or repoURL. Please provide just one of these values")
		os.Exit(2)
	} else if thogEntropy && !(toolName == "all" || toolName == "thog") {
		fmt.Println("thogEntropy flag should be used only when thog is being run. So, either leave the toolName blank or the toolName should be thog")
		os.Exit(2)
	} else if enterpriseURL == "" && (repoURL != "" || gistURL != "") {
		var ed, url string

		if repoURL != "" {
			url = repoURL
		} else if gistURL != "" {
			url = gistURL
		}

		if strings.Split(strings.Split(url, ":")[0], "@")[0] == "git" {
			fmt.Println("SSH URL")
			ed = strings.Split(strings.Split(url, ":")[0], "@")[1]
		} else if strings.Split(url, "/")[0] == "https:" {
			fmt.Println("HTTPS URL")
			ed = strings.Split(url, "/")[2]
		}

		matched, err := regexp.MatchString("github.com", ed)
		check(err)

		if !matched {
			fmt.Println("By the domain provided in the repoURL/gistURL, it looks like you are trying to scan a Github Enterprise repo/gist. Therefore, you need to provide the enterpriseURL flag as well")
			os.Exit(2)
		}
	} else if teamName != "" && org == "" {
		fmt.Println("Can't have a teamName without an org! Please provide a value for org along with the team name")
		os.Exit(2)
	} else if orgOnly && org == "" {
		fmt.Println("orgOnly flag should be used with a valid org")
		os.Exit(2)
	} else if scanPrivateReposOnly && user == "" && repoURL == "" && org == "" {
		fmt.Println("scanPrivateReposOnly flag should be used along with either the user, org or the repoURL")
		os.Exit(2)
	} else if scanPrivateReposOnly && (user != "" || repoURL != "" || org != "") {
		fmt.Println("scanPrivateReposOnly flag is provided with either the user, the repoURL or the org")

		err := checkifsshkeyexists()
		check(err)

		//Authenticating to Github using the token
		ctx1 := context.Background()
		client1, err := authenticatetogit(ctx1, token)
		check(err)

		if user != "" || repoURL != "" {
			var userRepos []*github.Repository
			opt3 := &github.RepositoryListOptions{
				Affiliation: "owner",
				ListOptions: github.ListOptions{PerPage: 10},
			}

			for {
				uRepos, resp, err := client1.Repositories.List(ctx1, "", opt3)
				check(err)
				userRepos = append(userRepos, uRepos...) //adding to the userRepos array
				if resp.NextPage == 0 {
					break
				}
				opt3.Page = resp.NextPage
			}

			if user != "" {
				fmt.Println("scanPrivateReposOnly flag is provided along with the user")
				fmt.Println("Checking to see if the token provided belongs to the user or not..")

				if *userRepos[0].Owner.Login == user {
					fmt.Println("Token belongs to the user")
				} else {
					fmt.Println("Token does not belong to the user. Please provide the correct token for the user mentioned.")
					os.Exit(2)
				}

			} else if repoURL != "" {
				fmt.Println("scanPrivateReposOnly flag is provided along with the repoURL")
				fmt.Println("Checking to see if the repo provided belongs to the user or not..")
				val, err := stringInSlice(repoURL, userRepos)
				check(err)
				if val {
					fmt.Println("Repo belongs to the user provided")
				} else {
					fmt.Println("Repo does not belong to the user whose token is provided. Please provide a valid repoURL that belongs to the user whose token is provided.")
					os.Exit(2)
				}
			}
		} else if org != "" && teamName == "" {
			var orgRepos []*github.Repository

			opt3 := &github.RepositoryListByOrgOptions{
				Type:        "private",
				ListOptions: github.ListOptions{PerPage: 10},
			}

			for {
				repos, resp, err := client1.Repositories.ListByOrg(ctx1, org, opt3)
				check(err)
				orgRepos = append(orgRepos, repos...)
				if resp.NextPage == 0 {
					break
				}
				opt3.Page = resp.NextPage
			}

			fmt.Println("scanPrivateReposOnly flag is provided along with the org")
			fmt.Println("Checking to see if the token provided belongs to a user in the org or not..")

			var i int
			if i >= 0 && i < len(orgRepos) {
				fmt.Println("Private Repos exist in this org and token belongs to a user in this org")
			} else {
				fmt.Println("Even though the token belongs to a user in this org, there are no Private repos in this org")
				os.Exit(2)
			}

		}

	} else if scanPrivateReposOnly && gistURL != "" {
		fmt.Println("scanPrivateReposOnly flag should NOT be provided with the gistURL since its a private repository or multiple private repositories that we are looking to scan. Please provide either a user, an org or a private repoURL")
		os.Exit(2)
	} else if !(toolName == "thog" || toolName == "repo-supervisor" || toolName == "all") {
		fmt.Println("Please enter either thog or repo-supervisor. Default is all.")
		os.Exit(2)
	} else if repoURL != "" && !scanPrivateReposOnly && enterpriseURL == "" {
		if strings.Split(repoURL, "@")[0] == "git" {
			fmt.Println("Since the repoURL is a SSH URL and no enterprise URL is provided, it is required to have the scanPrivateReposOnly flag and the SSH key mounted on a volume")
			os.Exit(2)
		}
	} else if enterpriseURL != "" {
		fmt.Println("Since enterpriseURL is provided, checking to see if the SSH key is also mounted or not")

		err := checkifsshkeyexists()
		check(err)
	}

	return nil
}

func makeDirectories() error {
	os.MkdirAll("/tmp/repos/org", 0700)
	os.MkdirAll("/tmp/repos/team", 0700)
	os.MkdirAll("/tmp/repos/users", 0700)

	return nil
}

func fileExists(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

func findTeamByName(ctx context.Context, client *github.Client, org string, teamName string) (*github.Team, error) {

	listTeamsOpts := &github.ListOptions{
		PerPage: 10,
	}
	Info("Listing teams...")
	for {
		teams, resp, err := client.Organizations.ListTeams(ctx, org, listTeamsOpts)
		check(err)
		//check the name here--try to avoid additional API calls if we've found the team
		for _, team := range teams {
			if *team.Name == teamName {
				return team, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		listTeamsOpts.Page = resp.NextPage
	}
	return nil, nil
}

func cloneTeamRepos(ctx context.Context, client *github.Client, org string, teamName string) error {

	// var team *github.Team
	team, err := findTeamByName(ctx, client, org, teamName)

	if team != nil {
		Info("Cloning the repositories of the team: " + *team.Name + "(" + strconv.FormatInt(*team.ID, 10) + ")")
		var teamRepos []*github.Repository
		listTeamRepoOpts := &github.ListOptions{
			PerPage: 10,
		}

		Info("Listing team repositories...")
		for {
			repos, resp, err := client.Organizations.ListTeamRepos(ctx, *team.ID, listTeamRepoOpts)
			check(err)
			teamRepos = append(teamRepos, repos...) //adding to the repo array
			if resp.NextPage == 0 {
				break
			}
			listTeamRepoOpts.Page = resp.NextPage
		}

		var teamrepowg sync.WaitGroup

		//iterating through the repo array
		for _, repo := range teamRepos {
			teamrepowg.Add(1)
			go executeclone(repo, "/tmp/repos/team/"+*repo.Name, &teamrepowg)
		}

		teamrepowg.Wait()

	} else {
		fmt.Println("Unable to find the team '" + teamName + "'; perhaps the user is not a member?\n")
		if err != nil {
			fmt.Println("Error was:")
			fmt.Println(err)
		}
		os.Exit(2)
	}
	return nil
}

func scanTeamRepos(org string) error {
	err := scanDir("/tmp/repos/team/", org)
	check(err)
	return nil
}

func authenticatetogit(ctx context.Context, token string) (*github.Client, error) {
	var client *github.Client
	var err error

	//Authenticating to Github using the token
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	if *enterpriseURL == "" {
		client = github.NewClient(tc)
	} else if *enterpriseURL != "" {
		client, err = github.NewEnterpriseClient(*enterpriseURL, *enterpriseURL, tc)
		if err != nil {
			fmt.Printf("NewEnterpriseClient returned unexpected error: %v", err)
		}
	}
	return client, nil
}

func main() {

	//Parsing the flags
	flag.Parse()

	executionQueue = make(chan bool, *threads)

	//Logic to check the program is ingesting proper flags
	err := checkflags(*token, *org, *user, *repoURL, *gistURL, *teamName, *scanPrivateReposOnly, *orgOnly, *toolName, *enterpriseURL, *thogEntropy)
	check(err)

	ctx := context.Background()

	//authN
	client, err := authenticatetogit(ctx, *token)
	check(err)

	//Creating some temp directories to store repos & results. These will be deleted in the end
	err = makeDirectories()
	check(err)

	//By now, we either have the org, user, repoURL or the gistURL. The program flow changes accordingly..

	if *org != "" { //If org was supplied
		m := "Since org was provided, the tool will proceed to scan all the org repos, then all the user repos and user gists in a recursive manner"

		if *orgOnly {
			m = "Org was specified combined with orgOnly, the tool will proceed to scan only the org repos and nothing related to its users"
		}

		Info(m)

		//cloning all the repos of the org
		err := cloneorgrepos(ctx, client, *org)
		check(err)

		if *teamName != "" { //If team was supplied
			Info("Since team name was provided, the tool will clone all repos to which the team has access")

			//cloning all the repos of the team
			err := cloneTeamRepos(ctx, client, *org, *teamName)
			check(err)

		}

		//getting all the users of the org into the allUsers array
		allUsers, err := listallusers(ctx, client, *org)
		check(err)

		if !*orgOnly {

			//iterating through the allUsers array
			for _, user := range allUsers {

				//cloning all the repos of a user
				err1 := cloneuserrepos(ctx, client, *user.Login)
				check(err1)

				//cloning all the gists of a user
				err2 := cloneusergists(ctx, client, *user.Login)
				check(err2)

			}
		}

		Info("Scanning all org repositories now..This may take a while so please be patient\n")
		err = scanorgrepos(*org)
		check(err)
		Info("Finished scanning all org repositories\n")

		if *teamName != "" { //If team was supplied
			Info("Scanning all team repositories now...This may take a while so please be patient\n")
			err = scanTeamRepos(*org)
			check(err)

			Info("Finished scanning all team repositories\n")
		}

		if !*orgOnly {

			Info("Scanning all user repositories and gists now..This may take a while so please be patient\n")
			var wguser sync.WaitGroup
			for _, user := range allUsers {
				wguser.Add(1)
				go scanforeachuser(*user.Login, &wguser)
			}
			wguser.Wait()
			Info("Finished scanning all user repositories and gists\n")
		}

	} else if *user != "" { //If user was supplied
		Info("Since user was provided, the tool will proceed to scan all the user repos and user gists\n")
		err1 := cloneuserrepos(ctx, client, *user)
		check(err1)

		err2 := cloneusergists(ctx, client, *user)
		check(err2)

		Info("Scanning all user repositories and gists now..This may take a while so please be patient\n")
		var wguseronly sync.WaitGroup
		wguseronly.Add(1)
		go scanforeachuser(*user, &wguseronly)
		wguseronly.Wait()
		Info("Finished scanning all user repositories and gists\n")

	} else if *repoURL != "" || *gistURL != "" { //If either repoURL or gistURL was supplied

		var url, repoorgist, fpath, rn, lastString, orgoruserName string
		var splitArray []string
		var bpath = "/tmp/repos/"

		if *repoURL != "" { //repoURL
			if *enterpriseURL != "" && strings.Split(strings.Split(*repoURL, "/")[0], "@")[0] != "git" {
				url = "git@" + strings.Split(*repoURL, "/")[2] + ":" + strings.Split(*repoURL, "/")[3] + "/" + strings.Split(*repoURL, "/")[4]
			} else {
				url = *repoURL
			}
			repoorgist = "repo"
		} else { //gistURL
			if *enterpriseURL != "" && strings.Split(strings.Split(*gistURL, "/")[0], "@")[0] != "git" {
				url = "git@" + strings.Split(*gistURL, "/")[2] + ":" + strings.Split(*gistURL, "/")[3] + "/" + strings.Split(*gistURL, "/")[4]
			} else {
				url = *gistURL
			}
			repoorgist = "gist"
		}

		Info("The tool will proceed to clone and scan: " + url + " only\n")

		if *enterpriseURL == "" && strings.Split(strings.Split(*gistURL, "/")[0], "@")[0] == "git" {
			splitArray = strings.Split(url, ":")
			lastString = splitArray[len(splitArray)-1]
		} else {
			splitArray = strings.Split(url, "/")
			lastString = splitArray[len(splitArray)-1]
		}

		if !*scanPrivateReposOnly {
			if *enterpriseURL != "" {
				orgoruserName = strings.Split(splitArray[0], ":")[1]
			} else {
				if *enterpriseURL == "" && strings.Split(strings.Split(*gistURL, "/")[0], "@")[0] == "git" {
					orgoruserName = splitArray[1]
				} else {
					orgoruserName = splitArray[3]
				}
			}
		} else {
			orgoruserName = strings.Split(splitArray[0], ":")[1]
		}

		switch repoorgist {
		case "repo":
			rn = strings.Split(lastString, ".")[0]
		case "gist":
			rn = lastString
		}
		fpath = bpath + orgoruserName + "/" + rn

		//cloning
		Info("Starting to clone: " + url + "\n")
		var wgo sync.WaitGroup
		wgo.Add(1)
		func(url string, fpath string, wgo *sync.WaitGroup) {
			enqueueJob(func() {
				gitclone(url, fpath, wgo)
			})
		}(url, fpath, &wgo)
		wgo.Wait()
		Info("Cloning of: " + url + " finished\n")

		//scanning
		Info("Starting to scan: " + url + "\n")
		var wgs sync.WaitGroup
		wgs.Add(1)

		func(rn string, fpath string, wgs *sync.WaitGroup, orgoruserName string) {
			enqueueJob(func() {
				runGitTools(*toolName, fpath+"/", wgs, rn, orgoruserName)
			})
		}(rn, fpath, &wgs, orgoruserName)

		wgs.Wait()
		Info("Scanning of: " + url + " finished\n")

	}

	//Now, that all the scanning has finished, time to combine the output
	// There are two option here:
	if *mergeOutput {
		// The first is to merge everything in /tmp/results into one JSON file
		Info("Merging the output into one JSON file\n")
		mergeOutputJSON(*outputFile)
	} else {
		// The second is to just concat the outputs
		Info("Combining the output into one file\n")
		err = combineOutput(*toolName, *outputFile)
		check(err)
	}
}
