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
	"strconv"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	uuid "github.com/satori/go.uuid"
)

var (
	org         = flag.String("org", "", "Name of the Organization to scan. Example: secretorg123")
	token       = flag.String("token", "", "Github Personal Access Token. This is required.")
	outputFile  = flag.String("output", "results.txt", "Output file to save the results.")
	user        = flag.String("user", "", "Name of the Github user to scan. Example: secretuser1")
	repoURL     = flag.String("repoURL", "", "HTTPS URL of the Github repo to scan. Example: https://github.com/anshumantestorg/repo1.git")
	gistURL     = flag.String("gistURL", "", "HTTPS URL of the Github gist to scan. Example: https://gist.github.com/secretuser1/81963f276280d484767f9be895316afc")
	cloneForks  = flag.Bool("cloneForks", false, "Option to clone org and user repos that are forks. Default is false")
	orgOnly     = flag.Bool("orgOnly", false, "Option to skip cloning user repo's when scanning an org. Default is false")
	toolName    = flag.String("toolName", "all", "Specify whether to run gitsecrets, thog or repo-supervisor")
	teamName    = flag.String("teamName", "", "Name of the Organization Team which has access to private repositories for scanning.")
	scanPrivate = flag.Bool("scanPrivate", false, "Option to scan private repositories. Default is false")
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

// Moving cloning logic out of individual functions
func executeclone(repo *github.Repository, directory string, wg *sync.WaitGroup) {
	urlToClone := ""
	switch *scanPrivate {
	case false:
		urlToClone = *repo.CloneURL
	case true:
		urlToClone = *repo.SSHURL
	default:
		urlToClone = *repo.CloneURL
	}

	var orgclone sync.WaitGroup
	// do not clone forks
	if !*cloneForks && *repo.Fork {
		fmt.Println(*repo.Name + " is a fork and the cloneFork flag was set to false so moving on..")
	} else {
		// clone it
		orgclone.Add(1)
		fmt.Println(urlToClone)
		// thread out the git clone
		go gitclone(urlToClone, directory, &orgclone)
	}

	orgclone.Wait()
	fmt.Println("")
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

	var orgrepowg sync.WaitGroup

	//iterating through the repo array
	for _, repo := range orgRepos {
		orgrepowg.Add(1)
		go executeclone(repo, "/tmp/repos/org/"+*repo.Name, &orgrepowg)
	}

	orgrepowg.Wait()
	fmt.Println("")
	return nil
}

func cloneuserrepos(ctx context.Context, client *github.Client, user string) error {
	Info("Cloning " + user + "'s repositories")

	var uname string

	if *scanPrivate {
		uname = ""
	} else {
		uname = user
	}

	var userRepos []*github.Repository
	opt3 := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 10},
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
	fmt.Println("")
	return nil
}

func cloneusergists(ctx context.Context, client *github.Client, user string) error {
	Info("Cloning " + user + "'s gists")

	var uname2 string

	if *scanPrivate {
		uname2 = ""
	} else {
		uname2 = user
	}

	var userGists []*github.Gist
	opt4 := &github.GistListOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}
	for {
		uGists, resp, err := client.Gists.List(ctx, uname2, opt4)
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

func runGitsecrets(filepath string, reponame string, orgoruser string) error {
	outputFile2 := "/tmp/results/gitsecrets/" + orgoruser + "_" + reponame + "_" + uuid.NewV4().String() + ".txt"
	cmd2 := exec.Command("./rungitsecrets.sh", filepath, outputFile2)
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	err2 := cmd2.Run()
	check(err2)
	return nil
}

func runTrufflehog(filepath string, reponame string, orgoruser string) error {
	outputFile1 := "/tmp/results/thog/" + orgoruser + "_" + reponame + "_" + uuid.NewV4().String() + ".txt"
	cmd1 := exec.Command("./thog/truffleHog/truffleHog.py", "-o", outputFile1, filepath)
	var out1 bytes.Buffer
	cmd1.Stdout = &out1
	err1 := cmd1.Run()
	check(err1)
	return nil
}

func runReposupervisor(filepath string, reponame string, orgoruser string) error {
	outputFile3 := "/tmp/results/repo-supervisor/" + orgoruser + "_" + reponame + "_" + uuid.NewV4().String() + ".txt"
	cmd3 := exec.Command("./runreposupervisor.sh", filepath, outputFile3)
	var out3 bytes.Buffer
	cmd3.Stdout = &out3
	err3 := cmd3.Run()
	check(err3)
	return nil
}

func runGitTools(tool string, filepath string, wg *sync.WaitGroup, reponame string, orgoruser string) {

	switch tool {
	case "all":
		err := runGitsecrets(filepath, reponame, orgoruser)
		check(err)
		err = runTrufflehog(filepath, reponame, orgoruser)
		check(err)
		err = runReposupervisor(filepath, reponame, orgoruser)
		check(err)

	case "gitsecrets":
		err := runGitsecrets(filepath, reponame, orgoruser)
		check(err)

	case "thog":
		err := runTrufflehog(filepath, reponame, orgoruser)
		check(err)

	case "repo-supervisor":
		err := runReposupervisor(filepath, reponame, orgoruser)
		check(err)
	}

	wg.Done()
}

func scanforeachuser(user string, wg *sync.WaitGroup) {
	var wguserrepogist sync.WaitGroup
	gituserrepos, _ := ioutil.ReadDir("/tmp/repos/users/" + user)
	for _, f := range gituserrepos {
		wguserrepogist.Add(1)
		go runGitTools(*toolName, "/tmp/repos/users/"+user+"/"+f.Name()+"/", &wguserrepogist, f.Name(), user)

	}
	wguserrepogist.Wait()
	wg.Done()
}

func toolsOutput(toolname string, of *os.File) error {

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

func singletoolOutput(toolname string, of *os.File) error {

	results, _ := ioutil.ReadDir("/tmp/results/" + toolname + "/")
	for _, f := range results {
		file, err := os.Open("/tmp/results/" + toolname + "/" + f.Name())
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
		tools := []string{"thog", "gitsecrets", "repo-supervisor"}

		for _, tool := range tools {
			err = toolsOutput(tool, of)
			check(err)
		}
	case "gitsecrets":
		err = singletoolOutput("gitsecrets", of)
		check(err)
	case "thog":
		err = singletoolOutput("thog", of)
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

// Moving directory scanning logic out of individual functions
func scanDir(dir string, org string) error {
	var wg sync.WaitGroup

	allRepos, _ := ioutil.ReadDir(dir)
	for _, f := range allRepos {
		wg.Add(1)
		go runGitTools(*toolName, dir+f.Name()+"/", &wg, f.Name(), org)

	}
	wg.Wait()
	return nil
}

func scanorgrepos(org string) error {
	err := scanDir("/tmp/repos/org/", org)
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

func checkflags(token string, org string, user string, repoURL string, gistURL string, teamName string, scanPrivate bool, orgOnly bool) error {
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
	} else if teamName != "" && org == "" {
		fmt.Println("Can't have a teamName without an org! Please provide a value for org along with the team name")
		os.Exit(2)
	} else if orgOnly && org == "" {
		fmt.Println("orgOnly flag should be used with a valid org")
		os.Exit(2)
	} else if scanPrivate && user == "" && repoURL == "" {
		fmt.Println("scanPrivate flag should be used along with either the user or the repoURL")
		os.Exit(2)
	} else if scanPrivate && (user != "" || repoURL != "") {
		fmt.Println("scanPrivate flag is provided with either the user or the repoURL")
		fmt.Println("Checking to see if the SSH key exists or not..")

		fi, err := os.Stat("/root/.ssh/id_rsa")
		if err == nil && fi.Size() > 0 {
			fmt.Println("SSH key exists and file size > 0 so continuing..")
		}
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}

		//Authenticating to Github using the token
		ctx1 := context.Background()
		ts1 := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc1 := oauth2.NewClient(ctx1, ts1)
		client1 := github.NewClient(tc1)

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
			fmt.Println("scanPrivate flag is provided along with the user")
			fmt.Println("Checking to see if the token provided belongs to the user or not..")

			if *userRepos[0].Owner.Login == user {
				fmt.Println("Token belongs to the user")
			} else {
				fmt.Println("Token does not belong to the user. Please provide the correct token for the user mentioned.")
				os.Exit(2)
			}

		} else if repoURL != "" {
			fmt.Println("scanPrivate flag is provided along with the repoURL")
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

	} else if scanPrivate && (org != "" || gistURL != "") {
		fmt.Println("scanPrivate flag should not be provided with either the org or the gistURL since its a private repository or multiple private repositories that we are looking to scan. Please provide either a user or a private repoURL")
		os.Exit(2)
	}

	return nil
}

func makeDirectories() error {
	os.MkdirAll("/tmp/repos/org", 0700)
	os.MkdirAll("/tmp/repos/team", 0700)
	os.MkdirAll("/tmp/repos/users", 0700)
	os.MkdirAll("/tmp/repos/singlerepo", 0700)
	os.MkdirAll("/tmp/repos/singlegist", 0700)
	os.MkdirAll("/tmp/results/thog", 0700)
	os.MkdirAll("/tmp/results/gitsecrets", 0700)
	os.MkdirAll("/tmp/results/repo-supervisor", 0700)

	return nil
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
		Info("Cloning the repositories of the team: " + *team.Name + "(" + strconv.Itoa(*team.ID) + ")")
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
		fmt.Println("")

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

func main() {

	//Parsing the flags
	flag.Parse()

	//Logic to check the program is ingesting proper flags
	err := checkflags(*token, *org, *user, *repoURL, *gistURL, *teamName, *scanPrivate, *orgOnly)
	check(err)

	//Authenticating to Github using the token
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

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
		var bpath = "/tmp/repos/"

		if *repoURL != "" { //repoURL
			url = *repoURL
			repoorgist = "repo"
		} else { //gistURL
			url = *gistURL
			repoorgist = "gist"
		}

		Info("The tool will proceed to clone and scan: " + url + " only\n")

		splitArray := strings.Split(url, "/")
		lastString = splitArray[len(splitArray)-1]

		if !*scanPrivate {
			orgoruserName = splitArray[3]
		} else {
			tempstr := splitArray[0]
			tempstr2 := strings.Split(tempstr, ":")
			orgoruserName = tempstr2[1]
		}

		switch repoorgist {
		case "repo":
			rn = strings.Split(lastString, ".")[0]
			fpath = bpath + "singlerepo/" + rn
		case "gist":
			rn = lastString
			fpath = bpath + "singlegist/" + lastString
		}

		//cloning
		Info("Starting to clone: " + url + "\n")
		var wgo sync.WaitGroup
		wgo.Add(1)
		go gitclone(url, fpath, &wgo)
		wgo.Wait()
		Info("Cloning of: " + url + " finished\n")

		//scanning
		Info("Starting to scan: " + url + "\n")
		var wgs sync.WaitGroup
		wgs.Add(1)

		go runGitTools(*toolName, fpath+"/", &wgs, rn, orgoruserName)

		wgs.Wait()
		Info("Scanning of: " + url + " finished\n")

	}

	//Now, that all the scanning has finished, time to combine the output
	Info("Combining the output into one file\n")
	err = combineOutput(*toolName, *outputFile)
	check(err)

}
