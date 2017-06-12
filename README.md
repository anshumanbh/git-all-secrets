# git-all-secrets



## About
git-all-secrets is a tool that can:
* Clone multiple public github repositories of an organization and scan them,
* Clone multiple public github repositories of a user that belongs to an organization and scan them,
* Clone a single repository of an organization/user and scan it,
* Clone a single gist of a user and scan it
* All of the above together!! Oh yeah!! Simply provide an organization name and get all their secrets.

Scanning is done by multiple open source tools such as:
* [truffleHog](https://github.com/dxa4481/truffleHog) and
* [git-secrets](https://github.com/awslabs/git-secrets)

NOTE - More such tools can be added in future, if desired!

The final output from the tool combines the output from all files from all the tools into one consolidated output file


## Getting started
The easiest way to run `git-all-secrets` is via Docker and I highly recommend installing Docker if you don't already have it. Once you have Docker installed,
* Type `docker run --rm -it abhartiya/tools_gitallsecrets --help` to understand the different flags it can take as inputs.
* Once you know what you want to scan, type something like `docker run -it abhartiya/tools_gitallsecrets -token=<> -output=<> -org=<>`
* After the container finishes running, retrieve the container ID by typing `docker ps -a`.
* Once you have the container ID, get the results file from the container to the host by typing `docker cp <container-id>:/data/<output-file> .`

If you don't have docker and don't want to install it, you can still run git-all-secrets. However, you need to do a few things before that.
* You need to git clone this repository since the codebase for truffleHog inside this repository is slightly different from the main truffleHog codebase. After cloning, make sure you can run truffleHog by typing `./thog/truffleHog/truffleHog.py`
* You also need to git clone my fork of git-secrets from [here](https://github.com/anshumanbh/git-secrets) because that codebase is slightly different as well from the main git-secrets codebase. And, then install it like how you would install git-secrets.
* Finally, type `go run main.go --help` and `go run main.go -org=<> -token=<> -output=<>`


## Flags/Options
* -token = Github personal access token. We need this because unauthenticated requests to the Github API can hit the rate limiting pretty soon!
* -org = Name of the Organization to scan. This will scan all repos in the org + all the repos & gists of all users in the org.
* -user = Name of the User to scan. This will scan all the repos & gists of this user.
* -repoURL = HTTPS URL of the Repo to scan. This will scan this repository only.
* -gistURL = HTTPS URL of the Gist to scan. This will scan this gist only.
* -output = This is the name of the file where all the results will get stored. By default, this is `results.txt`

### Note
* The `token` flag is compulsory. This can't be empty.
* The `org`, `user`, `repoURL` and `gistURL` can't be all empty at the same time. You need to provide just one of these values. If you provide all of them or multiple values together, the order of precendence will be `org` > `user` > `repoURL` > `gistURL`. For instance, if you provide both the flags `-org=secretorg123` and `-user=secretuser1` together, the tool will complain that it doesn't need anything along with the `org` value. To run it against a particular user only, just need to provide the `user` flag and not the `org` flag.


## Demo
A short demo is here - https://www.youtube.com/watch?v=KMO0Mid3npQ&feature=youtu.be


## TODO
* Support scanning Github Enterprise


## Details
### Features
* It uses Golang and GoRoutines. Things like cloning of repositories, running the tools on each of those repositories are all multi-threaded so it makes it super fast. Give it a try!
* It also looks for Slack tokens that have the pattern `xoxp-` and `xoxb-`. Take a look at [this](https://labs.detectify.com/2016/04/28/slack-bot-token-leakage-exposing-business-critical-information/) article to understand why scanning these tokens are super important.
* As mentioned above, it also looks for users gists.
* If there is a new tool that is good, it can be integrated into `git-all-secrets` pretty effortlessly.
* It is built for integration with other tools and frameworks. It takes in a few input parameters and produces an output file of the results. Pretty straightforward!
* If there are new patterns that need to be added, adding those is pretty easy as well. Take a look at the file `rungitsecrets.sh` and check how the `xoxp-` and `xoxb-` patterns were added.
* It uses truffleHog and git-secrets with some modifications to their codebase to make the output much better. `truffleHog` doesn't output the results to a file so that has been added. `git-secrets` has a lot of unnecessary output even when no secret is found so some of that output is removed for better readability.

### Motivation
I looked at a large number of open source tools that could be potentially used to look for secrets in github repositories. Some of the top tools that I thought were good are: [gitrob](https://github.com/michenriksen/gitrob), [truffleHog](https://github.com/dxa4481/truffleHog) and [git-secrets](https://github.com/awslabs/git-secrets).

Gitrob is meant to be a standalone tool that is pretty difficult to integrate with other tools because it has its own database and UI to see all the secrets discovered. It also produces a ton of false positives, more than truffleHog. And, it doesn't really highlight the secrets discovered. It just looks at the files and their extensions, not the actual content. So, although Gitrob is a great tool to get started with, I would recommend running it every once in a while to understand what the attack surface looks like and see if it has changed.

Then, there is truffleHog that looks for secrets in the actual contents of the file by looking at Shannon's entropy and prints the output on the screen. It takes in a repository URL or a repository directory as an argument. This is a pretty good tool although it does have its share of false positives. Some of the other drawbacks are:
* We can't use it recursively to scan directories that contain multiple repositories.
* There is no way we can use truffleHog to identify secrets that follow a certain pattern but don't have a high enough entropy i.e. we can't make it look for secrets that we know of but not necessarily have high entropy to be considered as a secret.
* It prints the output on the screen so not really useful for automation as such.

Finally, there is git-secrets which can flag things like AWS secrets. The best part is that you can add your own regular expressions as well for secrets that you know it should be looking for. A major drawback is that it doesn't do a good job on finding high entropy strings like truffleHog does. You can also only scan a particular directory that is a repository so no recursion scanning from a directory of repositories either.

So, as you can see, there are decent tools out there, but they had to be combined somehow. There was also a need to recursively scan multiple repositories and not just one. And, what about gists? There are organizations and users. Then, there are repositories for organizations and users. There are also gists by users. All of these should be scanned. And, scanned such that it could be automated and easily consumed by other tools/frameworks.

