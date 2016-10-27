package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
)

type (
	Repo struct {
		Owner   string
		Name    string
		Link    string
		Avatar  string
		Branch  string
		Private bool
		Trusted bool
	}

	Build struct {
		Number   int
		Event    string
		Status   string
		Deploy   string
		Created  int64
		Started  int64
		Finished int64
		Link     string
	}

	Commit struct {
		Remote  string
		Sha     string
		Ref     string
		Link    string
		Branch  string
		Message string
		Author  Author
	}

	Author struct {
		Name   string
		Email  string
		Avatar string
	}

	Config struct {
		CatalogRepo     string
		GithubToken     string
		GithubUser      string
		GithubEmail     string
		TemplateName    string
		TemplateVersion string
	}

	Plugin struct {
		Repo   Repo
		Build  Build
		Commit Commit
		Config Config
	}

	catalog struct {
		plugin          Plugin
		config          Config
		repo            string
		repoDir         string
		workingDir      string
		TemplateVersion string
		TemplateName    string
	}
)

const (
	dockerComposeTemplateFile  string = "rancher_base/docker-compose.yml.tmpl"
	rancherComposeTemplateFile string = "rancher_base/rancher-compose.yml.tmpl"
	configTemplateFile         string = "rancher_base/config.yml.tmpl"
	iconFilerancher_base       string = "rancher_base/catalogIcon.png"
)

func (p Plugin) Exec() error {
	fmt.Println("starting drone-rancher-catalog...")

	var catalog = catalog{}
	catalog.plugin = p
	catalog.config = p.Config
	catalog.TemplateName = p.Config.TemplateName
	catalog.TemplateVersion = p.Config.TemplateVersion

	workingDir, _ := os.Getwd()
	catalog.workingDir = workingDir

	// Clone the catalog repo
	catalog.repoDir = catalog.cloneCatalogRepo()

	// Setup git for future commits
	catalog.gitConfigureEmail()
	catalog.gitConfigureUser()

	// Prepare directory structure
	catalog.createTemplateDir()

	// Create config files
	catalog.createConfigFile("docker-compose.yml", dockerComposeTemplateFile)
	catalog.createConfigFile("rancher-compose.yml", rancherComposeTemplateFile)
	catalog.createConfigFile("../config.yml", configTemplateFile)

	// Icon file
	copyIcon(iconFilerancher_base, catalog.getEntryTarget("../catalogIcon.png"))

	if catalog.gitChanged() {
		catalog.addCatalogRepo()
		catalog.commitCatalogRepo()
		catalog.pushCatalogRepo()
	}
	fmt.Println("... Finished drone-rancher-catalog")

	return nil
}

func (c *catalog) createConfigFile(target string, tmplFile string) {
	composeTmpl := c.parseTemplateFile(tmplFile)
	composeTarget := c.getEntryTarget(target)
	c.executeTemplate(composeTarget, composeTmpl)
}

func (c *catalog) getEntryDir() string {
	return fmt.Sprintf("%s/templates/%s/%v", c.repoDir, c.config.TemplateName, c.plugin.Build.Number)
}

func (c *catalog) getEntryTarget(target string) string {
	return fmt.Sprintf("%s/%s", c.getEntryDir(), target)
}

func (c *catalog) cloneCatalogRepo() string {
	githubURL := fmt.Sprintf("https://%s:x-oauth-basic@github.com/%s.git", c.config.GithubToken, c.config.CatalogRepo)

	fmt.Println("Cloning Rancher-Catalog repo:", c.config.CatalogRepo)

	// create a temp dir
	tmpDir, _ := ioutil.TempDir("", "rancher-catalog")
	repoDir := path.Join(tmpDir, "repo")

	cmd := exec.Command("git", "clone", githubURL, repoDir)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("ERROR: Failed to Clone Repo %v\n", err)
		os.Exit(1)
	}
	return repoDir
}

func (c *catalog) gitConfigureEmail() {
	os.Chdir(c.repoDir)
	cmd := exec.Command("git", "config", "user.email", c.config.GithubEmail)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("ERROR: Failed to git config %v\n", err)
		os.Exit(1)
	}
}

func (c *catalog) gitConfigureUser() {
	os.Chdir(c.repoDir)
	cmd := exec.Command("git", "config", "user.name", c.config.GithubUser)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("ERROR: Failed to git config %v\n", err)
		os.Exit(1)
	}
}

func (c *catalog) createTemplateDir() {
	os.MkdirAll(c.getEntryDir(), 0755)
}

func (c *catalog) parseTemplateFile(file string) *template.Template {
	os.Chdir(c.workingDir)
	name := filepath.Base(file)
	tmpl, err := template.New(name).ParseFiles(file)
	if err != nil {
		fmt.Printf("ERROR: Failed parse template %v\n", err)
		os.Exit(1)
	}
	return tmpl
}

func (c *catalog) executeTemplate(target string, tmpl *template.Template) {
	targetFile, err := os.Create(target)
	if err != nil {
		fmt.Printf("ERROR: Failed to open file %v\n", err)
		os.Exit(1)
	}
	err = tmpl.Execute(targetFile, c)
	if err != nil {
		fmt.Printf("ERROR: Failed execute template %v\n", err)
		os.Exit(1)
	}
	targetFile.Close()
}

func (c *catalog) addCatalogRepo() {
	os.Chdir(c.repoDir)
	cmd := exec.Command("git", "add", "-A")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("ERROR: Failed to git add %v\n", err)
		os.Exit(1)
	}
}

func (c *catalog) commitCatalogRepo() {
	os.Chdir(c.repoDir)
	message := fmt.Sprintf("'Update from Drone Build: %d'", c.plugin.Build.Number)
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("ERROR: Failed to git commit %v\n", err)
		os.Exit(1)
	}
}

func (c *catalog) pushCatalogRepo() {
	os.Chdir(c.repoDir)
	cmd := exec.Command("git", "push")
	err := cmd.Run()
	// Not showing output, bleeds the API key
	if err != nil {
		fmt.Printf("ERROR: Failed to git push %v\n", err)
		os.Exit(1)
	}
}

// returns true if there are files that need to be commited.
func (c *catalog) gitChanged() bool {
	os.Chdir(c.repoDir)
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("ERROR: Failed to git status %v\n", err)
		os.Exit(1)
	}
	// no output means no changes.
	if len(out) == 0 {
		fmt.Println("No files changed.")
		return false
	}
	fmt.Println("Files changed, add/commit/push changes.")
	return true
}

// copy src.* (repo/base/catalogIcon.*) to dest directory
func copy(src string, dest string) {
	cmd := exec.Command("cp", src, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("ERROR: Failed to cp %v\n", err)
		os.Exit(1)
	}
}

func copyIcon(src string, dest string) {
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	// find files in dir that match base
	iconRe := regexp.MustCompile(fmt.Sprintf(`^%s`, base))
	files, _ := ioutil.ReadDir(dir)
	for _, f := range files {
		if iconRe.MatchString(f.Name()) {
			name := fmt.Sprintf("%s/%s", dir, f.Name())
			copy(name, dest)
		}
	}
}
