package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

func main() {

	var repositories []string

	var repoFile, branch, since, until string

	var agregated, clone bool

	flag.StringVar(&repoFile, "repositories", "", "File containing new line separated list of repos")
	flag.StringVar(&branch, "branch", "develop", "Branch to analyze")
	flag.BoolVar(&agregated, "agregate", false, "Agregate all repos in on single report")
	flag.StringVar(&since, "since", "", "Begin date to analysis (YYYY/MM/dd)")
	flag.StringVar(&until, "until", "", "End date for analysis (YYYY/MM/dd)")
	flag.BoolVar(&clone, "remoteRepos", false, "Use remote repositories (Needs repo clone url as parameter)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n %s\n %s\n", os.Args[0], os.Args[0]+" [OPTIONS] [repository]*", os.Args[0]+" -remoteRepos [OPTIONS] [repository_URL]*")
		fmt.Fprintf(os.Stderr, "OPTIONS:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()

	if repoFile == "" {
		if len(args) > 0 {
			repositories = flag.Args()
		} else {
			log.Fatal("No repositories provided:  Execute 'analyzer -h' for help ")
		}
	} else {

		file, err := os.Open(repoFile)
		if err != nil {
			log.Fatal(err)
		} else {

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				repositories = append(repositories, scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
		}

		defer file.Close()
	}
	createAnalyticsDir()

	if clone {
		createTmpDir()
		os.Chdir("tmp")
		defer removeTmpDir()
	}

	runtime.GOMAXPROCS(10)

	log.Print("Checking access to provided repositories")
	for _, value := range repositories {
		cmd := "git"
		args := []string{"ls-remote", value, "foo"}
		command := exec.Command(cmd, args...)
		command.Stdin = os.Stdin
		writer := io.MultiWriter(os.Stdout)
		command.Stdout = writer
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			//fmt.Println("Not able to verify access")
			os.Exit(1)
		}
	}
	log.Print("Access granted to all provided repos")
	if !agregated {
		var wg sync.WaitGroup
		wg.Add(len(repositories))

		for _, value := range repositories {
			go func(value string) {
				defer wg.Done()
				processRepo(value, branch, since, until, clone)
			}(value)
		}
		wg.Wait()
	} else {
		processAgregatedRepos(repositories, branch, since, until, clone)
	}

}

func removeTmpDir() {
	os.Chdir("../")
	os.RemoveAll("tmp")
}

func createTmpDir() {
	if exists, _ := exists("tmp"); exists {
	} else {
		os.Mkdir("tmp", 777)
	}
}
func createAnalyticsDir() {
	if exists, _ := exists("analytics"); exists {
	} else {
		os.Mkdir("analytics", 777)
	}
}

func cloneRepo(repoName string, repo string, branch string, clone bool) {

	if clone {
		cmd := "git"
		args := []string{"clone", repo}
		fmt.Println("Cloning repo: ", repoName)
		cloneCommand := exec.Command(cmd, args...)
		cloneCommand.Stdin = os.Stdin
		writer := io.MultiWriter(os.Stdout)
		cloneCommand.Stdout = writer
		cloneCommand.Stderr = os.Stderr
		if err := cloneCommand.Run(); err != nil {
			os.Exit(1)
		}
		fmt.Println("Successfully cloned", repoName)
	}
	fmt.Println("Checking out branch", branch, "on repo", repoName)
	cmd := "git"
	args := []string{"--git-dir=" + repoName + "/.git", "--work-tree=" + repoName, "checkout", branch}
	checkoutCommand := exec.Command(cmd, args...)
	checkoutCommand.Stdin = os.Stdin
	checkoutCommand.Stdout = os.Stdout
	checkoutCommand.Stderr = os.Stderr
	if err := checkoutCommand.Run(); err != nil {
		os.Exit(1)
	}

}

func storeResults(fileName string, result []byte, clone bool) {
	var path string

	if clone {
		path = "../analytics/" + fileName + ".html"
	} else {
		path = "analytics/" + fileName + ".html"
	}

	f, _ := os.Create(path)
	defer f.Close()
	_, err := f.Write(result)
	f.Sync()
	if err != nil {
		fmt.Println(err)
	}

}

func processAgregatedRepos(repositories []string, branch string, since string, until string, clone bool) {
	var wg sync.WaitGroup

	cmd := "gitinspector.py"
	args := []string{"--format=html", "-Tmw"}

	fileName := "aggregated_repo"

	if since != "" {
		args = append(args, "--since="+since)
		fileName = fileName + "_SINCE_" + since
	}
	if until != "" {
		args = append(args, "--until="+until)
		fileName = fileName + "_UNTIL_" + until
	}

	wg.Add(len(repositories))

	for _, value := range repositories {
		go func(value string) {
			defer wg.Done()
			if clone {
				cloneRepo(getRepoName(value), value, branch, true)
			} else {
				cloneRepo(value, value, branch, false)
			}
		}(value)
		if clone {
			args = append(args, getRepoName(value))
		} else {
			args = append(args, value)
		}
	}
	wg.Wait()

	command := exec.Command(cmd, args...)
	command.Stdin = os.Stdin

	if result, err := command.CombinedOutput(); err != nil {
		fmt.Fprintln(os.Stderr, command.Stderr)
		os.Exit(1)
	} else {
		fmt.Println("Storing results for")
		fileName = strings.Replace(fileName, "/", "-", -1)
		storeResults(fileName, result, clone)

	}
	fmt.Println("Repos processed")

}

func processRepo(repo string, branch string, since string, until string, clone bool) {

	var repoName string

	if clone {
		repoName = getRepoName(repo)
	} else {
		repoName = repo
	}

	cloneRepo(repoName, repo, branch, clone)

	cmd := "gitinspector.py"
	args := []string{"--format=html", "-Tmw"}

	fileName := repoName

	if since != "" {
		args = append(args, "--since="+since)
		fileName = fileName + "_SINCE_" + since
	}
	if until != "" {
		args = append(args, "--until="+until)
		fileName = fileName + "_UNTIL_" + until
	}

	args = append(args, repoName)

	command := exec.Command(cmd, args...)
	command.Stdin = os.Stdin

	if result, err := command.CombinedOutput(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, command.Stderr)
		os.Exit(1)
	} else {
		fmt.Println("Storing results for " + repoName)
		fileName = strings.Replace(fileName, "/", "-", -1)
		storeResults(fileName, result, clone)

	}
	fmt.Println("Repo", repoName, " processed")

}

func getRepoName(repo string) string {
	c1 := exec.Command("echo", repo)
	c2 := exec.Command("sed", "s/.*[:/]\\([^:/]*\\)\\.git$/\\1/")

	c2.Stdin, _ = c1.StdoutPipe()
	result, _ := c2.StdoutPipe()
	_ = c1.Start()
	_ = c2.Start()
	_ = c1.Wait()
	repoName, _ := ioutil.ReadAll(result)
	_ = c2.Wait()

	return strings.TrimSpace(string(repoName))

}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
