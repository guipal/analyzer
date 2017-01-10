package main

import (
	"bufio"
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

	args := os.Args[1:]
	var repositories, branches []string

	file, err := os.Open(args[0])
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

	if len(args) < 2 {
		branches = append(branches, "develop")
	} else {

		fileBranches, err := os.Open(args[1])
		if err != nil {
			log.Fatal(err)
			branches = append(branches, "develop")
		} else {

			scanner := bufio.NewScanner(fileBranches)
			for scanner.Scan() {
				branches = append(branches, scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
		}

		defer fileBranches.Close()
	}
	createTmpDir()
	createAnalyticsDir()

	os.Chdir("tmp")

	runtime.GOMAXPROCS(10)
	var wg sync.WaitGroup
	wg.Add(len(repositories))

	for _, value := range repositories {
		go func(value string) {
			defer wg.Done()
			processRepo(value, branches)
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

func processRepo(repo string, branches []string) {

	repoName := getRepoName(repo)

	fmt.Println("Cloning repo: ", repoName)
	cmd := "git"
	args := []string{"clone", repo}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Successfully cloned", repoName)

	for _, value := range branches {

		fmt.Println("Evaluating branch", value, "on repo", repoName)
		cmd := "git"
		args := []string{"--git-dir=" + repoName + "/.git", "--work-tree=" + repoName, "checkout", value}
		if err := exec.Command(cmd, args...).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		cmd = "gitinspector.py"
		args = []string{"--format=html", "-Tmw", repoName}

		command := exec.Command(cmd, args...)
		command.Stdin = os.Stdin

		if result, err := command.CombinedOutput(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, command.Stderr)

			os.Exit(1)
		} else {
			fmt.Println("Storing results for " + repoName)
			f, _ := os.Create("../analytics/" + repoName + ".html")
			defer f.Close()
			_, err := f.Write(result)
			f.Sync()
			if err != nil {
				fmt.Println(err)
			}
		}
		fmt.Println("Evaluated branch", value, "on repo", repoName)

	}

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
