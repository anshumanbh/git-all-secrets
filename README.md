# git-all-secrets



## About
git-all-secrets is a tool that can:
* Clone multiple public/private github repositories of an organization and scan them,
* Clone multiple public/private github repositories of a user that belongs to an organization and scan them,
* Clone a single public/private repository of an organization and scan it,
* Clone a single public/private repository of a user and scan it,
* Clone a single public/secret gist of a user and scan it
* Clone a team's repositories in an organization and scan them,
* All of the above together!! Oh yeah!! Simply provide an organization name and get all their secrets. If you also want to get secrets of a team within an organization, just mention the team name along with the org.
* Clone and scan Github Enterprise repositories and gists as well.

Scanning is done by multiple open source tools such as:
* [truffleHog](https://github.com/dxa4481/truffleHog) - scans commits for high entropy strings and user provided regular expressions,
* [repo-supervisor](https://github.com/auth0/repo-supervisor) - scans for high entropy strings in .js and .json files

NOTE - More such tools can be added in future, if desired!
NOTE - Scanning can be done by all the tools or any one of them by specifying the `toolName` flag.

If all the tools are used to scan, the final output from the tool combines the output from all files from all the tools into one consolidated output file.


## Getting started
The easiest way to run `git-all-secrets` is via Docker and I highly recommend installing Docker if you don't already have it. Once you have Docker installed,
* Type `docker run --rm -it abhartiya/tools_gitallsecrets --help` to understand the different flags it can take as inputs.
* Once you know what you want to scan, type something like `docker run -it abhartiya/tools_gitallsecrets -token=<> -org=<>`. You can also specify a particular tool to use for scanning by typing something like `docker run -it abhartiya/tools_gitallsecrets -token=<> -org=<> -toolName=<>`. Options are `thog` and `repo-supervisor`.
* If you want to run truffleHog with the default regex AND the high entropy settings, provide the `thogEntropy` flag like this - `docker run -it abhartiya/tools_gitallsecrets -token=<> -org=<> -toolName=thog -thogEntropy`.
* After the container finishes running, retrieve the container ID by typing `docker ps -a`.
* Once you have the container ID, get the results file from the container to the host by typing `docker cp <container-id>:/root/results.txt .`


## Flags/Options
* -token = Github personal access token. We need this because unauthenticated requests to the Github API can hit the rate limiting pretty soon!

* -org = Name of the Organization to scan. This will scan all public repos in the org + all the repos & gists of all users in the org. If you are using a token of a user who is a part of this org, it will also clone and scan all the secret gists belonging to that user as well as all the private repos in that org that the user has access to. However, it will NOT clone and scan any private repositories of this user belonging to this org. To scan private repositories of users, please use the `scanPrivateReposOnly` flag with the `user` flag along with the SSH key mounted on a volume.

* -user = Name of the User to scan. This will scan all the repos & gists of this user. If the token provided is the token of the user, secret gists will also be cloned and scanned. But, only public repos will be cloned and scanned. To scan private repositories of this user, please use the `scanPrivateReposOnly` flag with the `user` flag along with the SSH key mounted on a volume.

* -repoURL = HTTPS URL of the Repo to scan. This will scan this repository only. For public repos, mentioning the `https` URL of the repo will suffice. However, if you wish to scan a private repo, then you need to provide the `ssh` URL along with the SSH key mounted on a volume and the `scanPrivateReposOnly` flag.

* -gistURL = HTTPS URL of the Gist to scan. This will scan this gist only. There is no concept of public or secret gist as long as you have the URL. Even if you have a secret gist, if someone knows the HTTPS URL of your secret gist, they can access it too.

* -output = This is the name of the file where all the results will get stored. By default, this is `results.txt`.

* -cloneForks = This is the optional boolean flag to clone forks of org and user repositories. By default, this is set to `0` i.e. no cloning of forks. If forks are to be cloned, this value needs to be set to `1`. Or, simply mention `-cloneForks` along with other flags.

* -orgOnly = This is the optional boolean flag to skip cloning user repositories belonging to an org. By default, this is set to `0` i.e. regular behavior. If user repo's are not to be scanned and only the org repositories are to be scanned, this value needs to be set to `1`. Or, simply mention `-orgOnly` along with other flags.

* -toolName = This is the optional string flag to specify which tool to use for scanning. By default, this is set to `all` i.e. thog and repo-supervisor will all be used for scanning. Values are either `thog` or `repo-supervisor`.

* -teamName = Name of the Organization Team which has access to private repositories for scanning. This flag is not fully tested so I can't guarantee the functionality.

* -scanPrivateReposOnly = This is the optional boolean flag to specify if you want to scan private user repositories or not. Mentioning this will NOT scan public user repositories. And, you need to provide the SSH key by mounting the volume onto the container. Also, this only works with either the `user` flag, the `repoURL` flag or the `org` flag.

    When the `org` flag is mentioned along with the `scanPrivateReposOnly` flag and without the `orgOnly` flag, it will scan the public AND the private repos belonging to this org to which the user has access to (whose token is provided). It will then continue to scan ONLY the private repositories of the user (whose token is provided). Finally, it will continue to scan all public and secret gists of this user (whose token is provided). In a nutshell, the `scanPrivateReposOnly` flag only really affects the `user` and the `repoURL` flag.

* -enterpriseURL = Optional flag to provide the enterprise Github URL, if you wish to scan enterprise repositories. It should be something like `https://github.org.com/api/v3` along with the SSH key mounted onto the container. Refer to [scanning github enterprise](#scanning-github-enterprise) below.

* -threads = Default value is `10`. This is to limit the number of threads if your system is not beefy enough. For the most part, leaving this to 10 should be okay.

* -thogEntropy = This is an optional flag that basically tells if you want to get back high entropy based secrets from truffleHog or not. The high entropy secrets from truffleHog produces a LOT of noise so if you don't really want all that noise and if you are running git-all-secrets on a big organization, I'd recommend not to mention this flag. By default, this is set to `False` which means truffleHog will only produce result based on the Regular expressions in the `rules.json` file. If you are scanning a fairly small org with a limited set of repos or a user with a few repos, mentioning this flag makes more sense.

* -mergeOutput = Optional flag to merge and deduplicate the ouput of the tools used (currently truffleHog and repo-supervisor). Default value is `False`.


### Note
* The `token` flag is compulsory. This can't be empty.

* The `org`, `user`, `repoURL` and `gistURL` can't be all empty at the same time. You need to provide just one of these values. If you provide all of them or multiple values together, the order of precendence will be `org` > `user` > `repoURL` > `gistURL`. For instance, if you provide both the flags `-org=secretorg123` and `-user=secretuser1` together, the tool will complain that it doesn't need anything along with the `org` value. To run it against a particular user only, just need to provide the `user` flag and not the `org` flag.

* When specifying the `scanPrivateReposOnly` flag:
    * One must mount a volume containing the private SSH key onto the Docker container using the `-v` flag.
    * It should be used anytime a private repository is scanned. Please use the `ssh` url when using the flag and not the `https` URL.
    * Please make sure the token being used actually belongs to the user whose private repository/gist you are trying to scan otherwise there will be errors.
    * The SSH key that you will be using should NOT have a passphrase set if you want this tool to work without any manual intervention.

    Refer to [scanning private repositories](#scanning-private-repositories) below.

* When specifying `teamName` it is important that the provided `token` belong to a user which is a member of the team. Unexpected results may occur otherwise. Refer to [scanning an organization team](#scanning-an-organization-team) below.

* When specifying the `enterpriseURL` flag, it will always consider the SSH url even if you provide the https url of a repository. All the enterprise cloning/scanning happens via the ssh url and not the https url.

* As mentioned above, make sure the SSH key being used (to scan the ssh URL) does not have any passphrase set.


## Scanning Private Repositories
The most secure way to scan private repositories is to clone using the SSH URLs. To accomplish this, one needs to place an appropriate SSH key which has been added to a Github User. Github has [helpful documentation](https://help.github.com/articles/adding-a-new-ssh-key-to-your-github-account/) for configuring your account. Make sure this key does not have any passphrase set on it. Once you have the SSH key, simply mount it to the Docker container via a volume. It is as simple as typing the below commands:

`docker run -it -v ~/.ssh/id_rsa_personal:/root/.ssh/id_rsa abhartiya/tools_gitallsecrets -token=<> -user=<> -scanPrivateReposOnly`

OR

`docker run -it -v ~/.ssh/id_rsa_personal:/root/.ssh/id_rsa abhartiya/tools_gitallsecrets -token=<> -repoURL=<> -scanPrivateReposOnly`

Here, I am mapping my personal SSH key `id_rsa_personal` stored locally to `/root/.ssh/id_rsa` inside the container so that git-all-secrets will try to clone the repo via `ssh` and will use the SSH key stored at `/root/.ssh/id_rsa` inside the container. This way, you are not really storing anything sensitive inside the container. You are just using a file from your local machine. Once the container is destroyed, it no longer has access to this key.


## Scanning an Organization Team
The Github API limits the circumstances where a private repository is reported. If one is trying to scan an Organization with a user which is not an admin, you may need to provide the team which provides repository access to the user. In order to do this, use the `teamName` flag along with the `org` flag. Example is below:

`docker run --it -v ~/.ssh/id_rsa_personal:/root/.ssh/id_rsa abhartiya/tools_gitallsecrets -token=<> -org=<> -teamName <>`


## Scanning Github Enterprise
git-all-secrets now supports scanning Github Enterprise as well. If you have your own Github Enterprise hosted behind a VPN or something, make sure you are connected on the VPN or on the correct network that has access to the Github Enterprise repos. The `enterpriseURL` is what you'd need to scan your Github Enterprise repos. Below are some examples:

Example 1:

`docker run -it -v ~/.ssh/id_rsa_gitenterprise:/root/.ssh/id_rsa -token <token> -enterpriseURL https://github.<org>.com/api/v3 -repoURL https://github.<org>.com/<user>/<repo>.git`

Here, I am now mounting my github enterprise SSH key onto the container, followed by my personal access token, the enterprise URL to which the requests will be sent and the repo I want to scan.

Example 2:

`docker run -it -v ~/.ssh/id_rsa_gitenterprise:/root/.ssh/id_rsa -token <token> -enterpriseURL https://github.<org>.com/api/v3 -repoURL https://github.<org>.com/<user>/<repo>.git -toolName thog -thogEntropy`

Above, I am now just running truffleHog against the repository with the Entropy settings.

Example 3:

`docker run -it -v ~/.ssh/id_rsa_gitenterprise:/root/.ssh/id_rsa -token <token> -enterpriseURL https://github.<org>.com/api/v3 -user <username> -scanPrivateReposOnly`

Above, I am scanning only the private repositories of the user whose token is provided with all the tools (repo-supevisor and thog), but without the entropy setting of truffleHog.


## TODO
* Test team scanning functionality
* ~~Fix the Goroutine bug~~ - Hopefully DONE!
* ~~Support scanning Github Enterprise~~ DONE!
* ~~Support cloning and scanning private repositories of an org~~ - DONE!
* ~~Replace gitsecrets by the new Regex functionality in truffleHog~~ - DONE!
* ~~Add support for scanning private user repositories via SSH keys~~ - DONE!
* ~~Add flag to avoid scanning forks~~ - DONE!


## Details
### Features
* You can add your own regular expressions in the `rules.json` file and include it when executing `docker run` using the argument `-v $(pwd)/rules.json:/root/truffleHog/rules.json`.
* The tool looks for some default regular expressions. If needed, it can also be made for high entropy strings. All this happens via the truffleHog tool.
* It can look for high entropy strings in .js and .json files via the repo-supervisor tool.
* It scans users gists, which most of the tools dont.
* If there is a new tool that is good, it can be integrated into `git-all-secrets` pretty effortlessly.
* It is built for integration with other tools and frameworks. It takes in a few input parameters and produces an output file of the results. Pretty straightforward!
* It supports scanning Github Enterprise orgs/users/repos/gists as well.
* Most of the tools out there are made to scan individual repositories. If you want to loop it over multiple repositories, you'd have to write your own for loop in a shell script or something like that. git-all-secrets can help you scan multiple repositories at one go.
* You can now merge outputs from both the tools into a json file which can then be used in other automation type tools/frameworks

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
* 7/31/18 - Made trugglehog's installation simpler by using `pip`. @mhmdiaa fixed a bug wrt trufflehog's output function where it wasn't merging and sorting properly. Updated the regex file to include things like `password`. Built and pushed a new Docker image. GLHF!

* 7/15/18 - Updated repo-supervisor's fork because the upstream had some changes. Rebuilt a new Docker image using the latest Trufflehog. Provided the rules.json file that contains all the regexes that Trufflehog uses to find secrets. Added the ability to also merge outputs (in json) for both the tools using the `-mergeOutput` flag. Drastically reduced the Docker image size by using multi-stage builds and dep for managing dependencies. Huge shout out to @mhmdiaa for all of this! 

* 12/12/17 - For some large repos, truffleHog fails and exits. But, we don't want to stop there. We want to notify the user that scanning failed for that repo and continue scanning the other repos. This is now implemented in the latest docker image.

* 12/11/17 - Removed gitsecrets because truffleHog supports regex functionality now. Simply, adding your regexes in the `rules.json` file and rebuilding the Docker image will basically give us the functionality that gitsecrets was giving previously so there is no need for gitsecrets anymore. I also added support for scanning Github Enterprise repos & gists. @high-stakes helped get a PR in that (hopefully) fixes the Goroutine bug by limiting the amount of threads. Finally, support for scanning private repositories for an organization was added as well.

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


### Donate
If you want to show some love, my BTC wallet address is `1PtMhXWCcMZCitcDfaEBe7jnV9sjKoNvq7`.
