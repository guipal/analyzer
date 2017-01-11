package main

import (
	"bufio"
	"flag"
	"fmt"
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

	flag.StringVar(&repoFile, "repositories", "", "File containing new line separated list of repos")
	flag.StringVar(&branch, "branch", "develop", "Branch to analyze")
	flag.StringVar(&since, "since", "", "Begin date to analysis (YYYY/MM/dd)")
	flag.StringVar(&until, "until", "", "End date for analysis (YYYY/MM/dd)")

	flag.Parse()

	args := flag.Args()

	if repoFile == "" {
		if len(args) > 0 {
			repoFile = args[0]
		} else {
			log.Fatal("No repositories provided:  Execute 'analyzer -h' for help ")
		}
	}

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

	createTmpDir()
	createAnalyticsDir()

	os.Chdir("tmp")

	runtime.GOMAXPROCS(10)
	var wg sync.WaitGroup
	wg.Add(len(repositories))

	for _, value := range repositories {
		go func(value string) {
			defer wg.Done()
			processRepo(value, branch, since, until)
		}(value)
	}
	wg.Wait()
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

func processRepo(repo string, branch string, since string, until string) {

	repoName := getRepoName(repo)

	fmt.Println("Cloning repo: ", repoName)
	cmd := "git"
	args := []string{"clone", repo}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Successfully cloned", repoName)

	fmt.Println("Evaluating branch", branch, "on repo", repoName)
	cmd = "git"
	args = []string{"--git-dir=" + repoName + "/.git", "--work-tree=" + repoName, "checkout", branch}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmd = "gitinspector.py"
	args = []string{"--format=html", "-Tmw"}

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

		f, _ := os.Create("../analytics/" + fileName + ".html")
		defer f.Close()
		_, err := f.Write(result)
		f.Sync()
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("Evaluated branch", branch, "on repo", repoName)

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
