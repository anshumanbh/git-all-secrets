# git-all-secrets



## About
git-all-secrets is a tool that can:
* Clone multiple public github repositories of an organization and scan them,
* Clone multiple public/private github repositories of a user that belongs to an organization and scan them,
* Clone a single public repository of an organization and scan it,
* Clone a single public/private repository of a user and scan it,
* Clone a single public/secret gist of a user and scan it
* Clone a team's repositories in an organization and scan them,
* All of the above together!! Oh yeah!! Simply provide an organization name and get all their secrets. If you also want to get secrets of a team within an organization, just mention the team name along with the org.

Scanning is done by multiple open source tools such as:
* [truffleHog](https://github.com/dxa4481/truffleHog) - scans commits for high entropy strings,
* [git-secrets](https://github.com/awslabs/git-secrets) - scans for things like AWS secrets, Slack tokens, and any other regular expressions you want to search for,
* [repo-supervisor](https://github.com/auth0/repo-supervisor) - scans for high entropy strings in .js and .json files

NOTE - More such tools can be added in future, if desired!
NOTE - Scanning can be done by all the tools or any one of them by specifying the `toolName` flag.

If all the tools are used to scan, the final output from the tool combines the output from all files from all the tools into one consolidated output file.


## Getting started
The easiest way to run `git-all-secrets` is via Docker and I highly recommend installing Docker if you don't already have it. Once you have Docker installed,
* Type `docker run --rm -it abhartiya/tools_gitallsecrets --help` to understand the different flags it can take as inputs.
* Once you know what you want to scan, type something like `docker run -it abhartiya/tools_gitallsecrets -token=<> -org=<>`. You can also specify a particular tool to use for scanning by typing something like `docker run -it abhartiya/tools_gitallsecrets -token=<> -org=<> -toolName=<>`. Options are `thog`, `repo-supervisor` and `gitsecrets`.
* After the container finishes running, retrieve the container ID by typing `docker ps -a`.
* Once you have the container ID, get the results file from the container to the host by typing `docker cp <container-id>:/data/results.txt .`

PS - Since I am going to keep adding more tools, supporting non-Docker running instructions is likely not going to happen! For instance, I just added the `repo-supervisor` tool to scan and setting it up was the biggest PITA ever. How much have I started to hate node and npm and nvm and blah..!

In other words, if you wish to use `git-all-secrets`, please use Docker! I have also uploaded the Docker image to my public Docker hub account. All you need to do is pull it and run it.


## Flags/Options
* -token = Github personal access token. We need this because unauthenticated requests to the Github API can hit the rate limiting pretty soon!

* -org = Name of the Organization to scan. This will scan all repos in the org + all the repos & gists of all users in the org. If you are using a token of a user who is a part of this org, it will also clone and scan all the secret gists beloning to that user. However, it will not clone and scan any private repositories of this user belonging to this org. To scan private repositories, please use the `scanPrivateReposOnly` flag with the `user` flag along with the SSH key mounted on a volume.

* -user = Name of the User to scan. This will scan all the repos & gists of this user. If the token provided is the token of the user, secret gists will also be cloned and scanned. But, only public repos will be cloned and scanned. To scan private repositories of this user, please use the `scanPrivateReposOnly` flag with the `user` flag along with the SSH key mounted on a volume.

* -repoURL = HTTPS URL of the Repo to scan. This will scan this repository only. For public repos, mentioning the `https` URL of the repo will suffice. However, if you wish to scan a private repo, then you need to provide the `ssh` URL along with the SSH key mounted on a volume and the `scanPrivateReposOnly` flag.

* -gistURL = HTTPS URL of the Gist to scan. This will scan this gist only. There is no concept of public or secret gist as long as you have the URL. Even if you have a secret gist, if someone knows the HTTPS URL of your secret gist, they can access it too.

* -output = This is the name of the file where all the results will get stored. By default, this is `results.txt`.

* -cloneForks = This is the optional boolean flag to clone forks of org and user repositories. By default, this is set to `0` i.e. no cloning of forks. If forks are to be cloned, this value needs to be set to `1`. Or, simply mention `-cloneForks` along with other flags.

* -orgOnly = This is the optional boolean flag to skip cloning user repositories belonging to an org. By default, this is set to `0` i.e. regular behavior. If user repo's are not to be scanned and only the org repositories are to be scanned, this value needs to be set to `1`. Or, simply mention `-orgOnly` along with other flags.

* -toolName = This is the optional string flag to specify which tool to use for scanning. By default, this is set to `all` i.e. gitsecrets, thog and repo-supervisor will all be used for scanning. Values are either `gitsecrets`, `thog` or `repo-supervisor`.

* -teamName = Name of the Organization Team which has access to private repositories for scanning. This flag is not fully tested so I can't guarantee the functionality.

* -scanPrivateReposOnly = This is the optional boolean flag to specify if you want to scan private repositories or not. It will NOT scan public repositories. And, you need to provide the SSH key by mounting the volume onto the container. Also, this only works with either the `user` flag or the `repoURL` flag. The scanning of private repos for organizations is not yet built.


### Note
* The `token` flag is compulsory. This can't be empty.

* The `org`, `user`, `repoURL` and `gistURL` can't be all empty at the same time. You need to provide just one of these values. If you provide all of them or multiple values together, the order of precendence will be `org` > `user` > `repoURL` > `gistURL`. For instance, if you provide both the flags `-org=secretorg123` and `-user=secretuser1` together, the tool will complain that it doesn't need anything along with the `org` value. To run it against a particular user only, just need to provide the `user` flag and not the `org` flag.

* When specifying `scanPrivateReposOnly` flag, one must mount a volume containing the private SSH key onto the Docker container. Refer to [scanning private repositories](#scanning-private-repositories) below.

* When specifying `teamName` it is important that the provided `token` belong to a user which is a member of the team. Unexpected results may occur otherwise. Refer to [scanning an organization team](#scanning-an-organization-team) below.

* When you mention the `scanPrivateReposOnly` flag, you want to make sure the token being used actually belongs to the user whose private repository/gist you are trying to scan otherwise there will be errors.

* The SSH key that you will be using should not have a passphrase set if you want this tool to work without any manual intervention.

* The tool works against the `api.github.com` domain only as of now. Support for Github Enterprise is being worked on.

* `scanPrivateReposOnly` flag should be used anytime a private repository is scanned. Please use the `ssh` url when using the flag.


## Scanning Private Repositories
The most secure way to scan private repositories is to clone using the SSH URLs. To accomplish this, one needs to place an appropriate SSH key which has been added to a Github User. Github has [helpful documentation](https://help.github.com/articles/adding-a-new-ssh-key-to-your-github-account/) for configuring your account. Once you have the SSH key, simply mount it to the Docker container via a volume. It is as simple as typing the below commands:

`docker run -it -v ~/.ssh/id_rsa_personal:/root/.ssh/id_rsa abhartiya/tools_gitallsecrets -token=<> -user=<> -scanPrivateReposOnly`

OR

`docker run -it -v ~/.ssh/id_rsa_personal:/root/.ssh/id_rsa abhartiya/tools_gitallsecrets -token=<> -repoURL=<> -scanPrivateReposOnly`

Here, I am mapping my personal SSH key `id_rsa_personal` stored locally to `/root/.ssh/id_rsa` inside the container so that git-all-secrets will try to clone the repo via `ssh` and will use the SSH key stored at `/root/.ssh/id_rsa` inside the container. This way, you are not really storing anything sensitive inside the container. You are just using a file from your local machine. Once the container is destroyed, it no longer has access to this key.


## Scanning an Organization Team
The Github API limits the circumstances where a private repository is reported. If one is trying to scan an Organization with a user which is not an admin, you may need to provide the team which provides repository access to the user. In order to do this, use the `teamName` flag along with the `org` flag. Example is below:

`docker run --it -v ~/.ssh/id_rsa_personal:/root/.ssh/id_rsa abhartiya/tools_gitallsecrets -token=<> -org=<> -teamName <>`


## Demo
A short demo is here - https://www.youtube.com/watch?v=KMO0Mid3npQ&feature=youtu.be


## TODO
* Support scanning Github Enterprise
* Support cloning and scanning private repositories of an org
* Test team scanning functionality
* Replace gitsecrets and repo-supervisor by truffleHog once https://github.com/dxa4481/truffleHog/issues/69 is fixed
* ~~Add support for scanning private user repositories via SSH keys~~ - DONE!
* ~~Add flag to avoid scanning forks~~ - DONE!


## Known Bugs
* I am aware of a bug with goroutines. This normally happens, when you try to scan a big org with a lot of users who have a lot of repositories. A lot of goroutines are spawned to do the scanning and if the machine is not beefy enough, the goroutines are going to complain. To solve this, the only most practical solution I can think of is to not scan a big org. Maybe, scan in batches. I am open to suggestions here! Try -orgOnly.


## Details
### Features
* It uses Golang and GoRoutines. Things like cloning of repositories, running the tools on each of those repositories are all multi-threaded so it makes it super fast. Give it a try!
* It also looks for Slack tokens that have the pattern `xoxp-` and `xoxb-`. Take a look at [this](https://labs.detectify.com/2016/04/28/slack-bot-token-leakage-exposing-business-critical-information/) article to understand why scanning these tokens are super important.
* As mentioned above, it also looks for users gists.
* If there is a new tool that is good, it can be integrated into `git-all-secrets` pretty effortlessly.
* It is built for integration with other tools and frameworks. It takes in a few input parameters and produces an output file of the results. Pretty straightforward!
* If there are new patterns that need to be added, adding those is pretty easy as well. Take a look at the file `rungitsecrets.sh` and check how the `xoxp-` and `xoxb-` patterns were added.
* It uses truffleHog and git-secrets with some modifications to their codebase to make the output much better. `truffleHog` doesn't output the results to a file so that has been added. `git-secrets` has a lot of unnecessary output even when no secret is found so some of that output is removed for better readability.
* It can be used to scan private user repositories by providing the SSH key.
* It can be used to scan team repositories belonging to an organization.

### Motivation
I looked at a large number of open source tools that could be potentially used to look for secrets in github repositories. Some of the top tools that I thought were good are: [gitrob](https://github.com/michenriksen/gitrob), [truffleHog](https://github.com/dxa4481/truffleHog) and [git-secrets](https://github.com/awslabs/git-secrets).

Gitrob is meant to be a standalone tool that is pretty difficult to integrate with other tools because it has its own database and UI to see all the secrets discovered. It also produces a ton of false positives, more than truffleHog. And, it doesn't really highlight the secrets discovered. It just looks at the files and their extensions, not the actual content. So, although Gitrob is a great tool to get started with, I would recommend running it every once in a while to understand what the attack surface looks like and see if it has changed.

Then, there is truffleHog that looks for secrets in the actual contents of the file by looking at Shannon's entropy and prints the output on the screen. It takes in a repository URL or a repository directory as an argument. This is a pretty good tool although it does have its share of false positives. Some of the other drawbacks are:
* We can't use it recursively to scan directories that contain multiple repositories.
* There is no way we can use truffleHog to identify secrets that follow a certain pattern but don't have a high enough entropy i.e. we can't make it look for secrets that we know of but not necessarily have high entropy to be considered as a secret.
* It prints the output on the screen so not really useful for automation as such.

Finally, there is git-secrets which can flag things like AWS secrets. The best part is that you can add your own regular expressions as well for secrets that you know it should be looking for. A major drawback is that it doesn't do a good job on finding high entropy strings like truffleHog does. You can also only scan a particular directory that is a repository so no recursion scanning from a directory of repositories either.

So, as you can see, there are decent tools out there, but they had to be combined somehow. There was also a need to recursively scan multiple repositories and not just one. And, what about gists? There are organizations and users. Then, there are repositories for organizations and users. There are also gists by users. All of these should be scanned. And, scanned such that it could be automated and easily consumed by other tools/frameworks.

### Changelog
* 12/08/17 - Removed my own fork of truffleHog. Using the upstream version now along with the *new* regex functionality of truffleHog + entropy mode. Soon, I believe we can replace both gitsecrets and repo supervisor by just truffleHog once some issues are fixed.

* 12/07/17 - I updated the documentation with some more details and explanation around the different flags.

* 12/05/17 - Integrated scanning support for private repositories via SSH key. This has been an ask for the longest time and it is now possible to do so. Also, changed the docker image tag scheme. From now on, the latest image will have the `latest` tag. And, all the previous versions will be tagged with a number. All this couldn't have been possible without the `SimpliSafe` team, specially Matthew Cox (https://github.com/matthew-cox). So, a big shoutout to you Matt!

* 10/14/17 - Built and pushed the new image abhartiya/tools_gitallsecrets:v6. This new image has the newer version of `git-secrets` as well as `repo-supervisor` i.e. I merged some upstream changes into my fork alongwith some additional changes I had already made in my fork. The new image uses these changes so everything is latest and greatest!

* 10/14/17 - Built and pushed the new image abhartiya/tools_gitallsecrets:v5. This image fixes a very stupid and irritating bug which was possibly causing repo supervisor to fail. Something changed in the way Environment values are being read in Dockerfile which resulted in repo supervisor not understanding which node path to use. Node hell!

* 9/29/17 - Built and pushed the new image with the `orgOnly` flag - abhartiya/tools_gitallsecrets:v4

* 8/22/17 - Added -orgOnly toggle by kciredor: analyzes specified organization repo and skips user repo's.

* 6/26/17 - Removed some output in repo-supevisor that printed out errors when there were no secrets found. Unnecessary output! Built and pushed the new image - abhartiya/tools_gitallsecrets:v3

* 6/25/17 - Added the flag `toolName` to specify which tool to use for scanning. Built and pushed the new image - abhartiya/tools_gitallsecrets:v2

* 6/14/17 - Added repo-supervisor as a scanning tool, also updated and added the version number to the docker image - abhartiya/tools_gitallsecrets:v1

* 6/14/17 - Added the flag cloneForks to avoid cloning forks of org and user repos. By default, this is false. If you wish to scan forks, just set the value to 1 i.e. -cloneForks=1
