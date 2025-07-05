package parser

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
)

var (
	ErrInvalidUsage = errors.New("invalid usage")
)

type Parser struct {
	Data        string
	Headers     []string
	Help        bool
	Interactive bool
	LogLevel    slog.Level
	Silent      bool
	Version     bool

	transportArgs []string

	tools     bool
	prompts   bool
	resources bool

	tool     string
	prompt   string
	resource string
}

func (p *Parser) Parse(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-T", "--tools":
			p.tools = true
		case "-P", "--prompts":
			p.prompts = true
		case "-R", "--resources":
			p.resources = true
		case "-h", "--help":
			p.Help = true
			return nil
		case "-I", "--interactive":
			p.Silent = true
			p.Interactive = true
		case "-s", "--silent":
			p.Silent = true
		case "-v", "--version":
			p.Version = true
			return nil
		default:
			switch arg {
			case "-t", "--tool", "-p", "--prompt", "-r", "--resource", "-d", "--data", "-H", "--header", "-l", "--log-level":
				if len(args) < i+2 {
					return ErrInvalidUsage
				}
				switch arg {
				case "-t", "--tool":
					p.tool = args[i+1]
				case "-p", "--prompt":
					p.prompt = args[i+1]
				case "-r", "--resource":
					p.resource = args[i+1]
				case "-d", "--data":
					data, err := p.ParseData(args[i+1])
					if err != nil {
						return fmt.Errorf("parse data: %w", err)
					}
					p.Data = data
				case "-H", "--header":
					headers, err := p.parseHeader(args[i+1])
					if err != nil {
						return fmt.Errorf("parse header: %w", err)
					}
					p.Headers = append(p.Headers, headers...)
				case "-l", "--log-level":
					if err := p.LogLevel.UnmarshalText([]byte(args[i+1])); err != nil {
						return fmt.Errorf("parse log level: %w", err)
					}
				}
				i++
			default:
				p.transportArgs = append(p.transportArgs, arg)
			}
		}
	}
	return nil
}

func (p *Parser) TransportArgs() []string {
	return slices.Clone(p.transportArgs)
}

func (p *Parser) Tools() bool {
	return p.tools
}

func (p *Parser) Prompts() bool {
	return p.prompts
}

func (p *Parser) Resources() bool {
	return p.resources
}

func (p *Parser) Tool() string {
	return p.tool
}

func (p *Parser) Prompt() string {
	return p.prompt
}

func (p *Parser) Resource() string {
	return p.resource
}

func (p Parser) ParseData(arg string) (string, error) {
	after, ok := strings.CutPrefix(arg, "@")
	if !ok {
		return after, nil
	}
	d, err := os.ReadFile(after)
	if err != nil {
		return "", fmt.Errorf("read data file: %w", err)
	}
	return strings.TrimSpace(string(d)), nil
}

func (p *Parser) parseHeader(header string) ([]string, error) {
	var ret []string
	after, ok := strings.CutPrefix(header, "@")
	if !ok {
		ret = append(ret, after)
		return ret, nil
	}
	file, err := os.Open(after)
	if err != nil {
		return nil, fmt.Errorf("read header file: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if h := strings.TrimSpace(scanner.Text()); h != "" {
			ret = append(ret, h)
		}
	}
	return ret, nil
}
