package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	ENV_LINE = `^(\S+)=(\S+)\s*$`

	//                     ----spec------------------------------------    --user--  -cmd-
	CRONJOB_SYSTEM = `^\s*([^@\s]+\s+\S+\s+\S+\s+\S+\s+\S+|@every\s+\S+)\s+([^\s]+)\s+(.+)$`

	//                  ----spec------------------------------------    -cmd-
	CRONJOB_USER = `^\s*([^@\s]+\s+\S+\s+\S+\s+\S+\s+\S+|@every\s+\S+)\s+(.+)$`

	CRONJOB_NAME = `^([\./\w]*\/)?([\w\s\.\&\|]+)(.*)?$`

	DEFAULT_SHELL = "sh"
)

var (
	envLineRegex       = regexp.MustCompile(ENV_LINE)
	cronjobSystemRegex = regexp.MustCompile(CRONJOB_SYSTEM)
	cronjobUserRegex   = regexp.MustCompile(CRONJOB_USER)
	cronjobNameRegex   = regexp.MustCompile(CRONJOB_NAME)
)

type EnvVar struct {
	Name  string
	Value string
}

type CrontabEntry struct {
	Name    string
	Spec    string
	User    string
	Command string
	Pwd     string
	Env     []string
	Shell   string
}

type Parser struct {
	cronLineRegex   *regexp.Regexp
	reader          io.Reader
	cronjobUsername string
}

// Create new crontab parser (user crontab without user specification)
func NewCronjobUserParser(reader io.Reader, username string) (*Parser, error) {
	p := &Parser{
		cronLineRegex:   cronjobUserRegex,
		reader:          reader,
		cronjobUsername: username,
	}

	return p, nil
}

// Create new crontab parser (crontab with user specification)
func NewCronjobSystemParser(reader io.Reader) (*Parser, error) {
	p := &Parser{
		cronLineRegex:   cronjobSystemRegex,
		reader:          reader,
		cronjobUsername: CRONTAB_TYPE_SYSTEM,
	}

	return p, nil
}

// Parse crontab
func (p *Parser) Parse() []CrontabEntry {
	entries := p.parseLines()

	return entries
}

// Parse lines from crontab
func (p *Parser) parseLines() []CrontabEntry {
	var entries []CrontabEntry
	var cronjobName string
	var crontabSpec string
	var crontabUser string
	var crontabCommand string
	var environment []string

	shell := DEFAULT_SHELL
	pwd := "/"

	specCleanupRegexp := regexp.MustCompile(`\s+`)

	scanner := bufio.NewScanner(p.reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// comment line
		if strings.HasPrefix(line, "#") {
			continue
		}

		// environment line
		if envLineRegex.MatchString(line) == true {
			m := envLineRegex.FindStringSubmatch(line)
			envName := strings.TrimSpace(m[1])
			envValue := strings.TrimSpace(m[2])

			if envName == "SHELL" {
				// custom shell for command
				shell = envValue
			} else if envName == "PWD" {
				pwd = envValue
			} else {
				// normal environment variable
				environment = append(environment, fmt.Sprintf("%s=%s", envName, envValue))
			}
		}

		// cronjob line
		if p.cronLineRegex.MatchString(line) == true {
			m := p.cronLineRegex.FindStringSubmatch(line)

			if p.cronjobUsername == CRONTAB_TYPE_SYSTEM {
				crontabSpec = strings.TrimSpace(m[1])
				crontabUser = strings.TrimSpace(m[2])
				crontabCommand = strings.TrimSpace(m[3])
			} else {
				crontabSpec = strings.TrimSpace(m[1])
				crontabUser = p.cronjobUsername
				crontabCommand = strings.TrimSpace(m[2])
			}

			if cronjobNameRegex.MatchString(crontabCommand) == true {
				command := cronjobNameRegex.FindStringSubmatch(crontabCommand)
				cronjobName = strings.TrimSpace(command[2])
			} else {
				cronjobName = crontabCommand
			}

			// shrink white spaces for better handling
			crontabSpec = specCleanupRegexp.ReplaceAllString(crontabSpec, " ")

			entries = append(entries, CrontabEntry{Name: cronjobName, Spec: crontabSpec, User: crontabUser,
				Command: crontabCommand, Pwd: pwd, Env: environment, Shell: shell})
		}
	}

	return entries
}
