package interactor

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/cherrydra/mcpurl/features"
	"github.com/cherrydra/mcpurl/parser"
	"github.com/cherrydra/mcpurl/transport"
	"github.com/cherrydra/mcpurl/version"
	"github.com/chzyer/readline"
	"github.com/google/shlex"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	ErrInvalidPipe = errors.New("invalid pipe command")
)

type Interactor struct {
	Session *mcp.ClientSession

	completer *mcpurlCompleter
}

func (i *Interactor) Run(ctx context.Context) error {
	i.completer = &mcpurlCompleter{ctx: ctx, s: &features.ServerFeatures{Session: i.Session}}
	l, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[36mmcpurl>\033[0m ",
		AutoComplete:    i.completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistoryFile:         historyFile(),
		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		return fmt.Errorf("create readline: %w", err)
	}
	defer l.Close()
	l.CaptureExitSignal()

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}
		if err := i.executeCommand(ctx, strings.TrimSpace(line)); err != nil {
			if errors.Is(err, parser.ErrInvalidUsage) {
				printUsage()
				continue
			}
			fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		}
	}
	if i.Session != nil {
		i.Session.Close()
	}
	return nil
}

func (ia *Interactor) executeCommand(ctx context.Context, command string) (err error) {
	// io redirect
	redirAppendParts := strings.Split(command, ">>")
	redirCreateParts := strings.Split(redirAppendParts[0], ">")
	var redirPart, redirMode string
	if len(redirAppendParts) > 1 {
		redirPart = strings.TrimSpace(redirAppendParts[len(redirAppendParts)-1])
		redirMode = "append"
	} else if len(redirCreateParts) > 1 {
		redirPart = strings.TrimSpace(redirCreateParts[len(redirCreateParts)-1])
		redirMode = "create"
	}
	stdout := os.Stdout
	switch redirMode {
	case "append":
		stdout, err = os.OpenFile(redirPart, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open file for append: %w", err)
		}
	case "create":
		stdout, err = os.Create(redirPart)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
	}

	// pipeline
	pipeParts := strings.Split(redirCreateParts[0], "|")

	var nextIn, out *os.File
	if len(pipeParts) > 1 {
		nextIn, out, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("create pipe: %w", err)
		}
	}

	errChan := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ia.executeMain(ctx, strings.TrimSpace(pipeParts[0]), cmp.Or(out, stdout)); err != nil {
			errChan <- err
		}
	}()
	for i, part := range pipeParts[1:] {
		thisIn := nextIn
		thisOut := stdout
		if i < len(pipeParts)-2 {
			nextIn, thisOut, err = os.Pipe()
			if err != nil {
				errChan <- fmt.Errorf("create pipe for part %d: %w", i+1, err)
				return
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ia.executePipe(ctx, strings.TrimSpace(part), thisIn, thisOut); err != nil {
				errChan <- fmt.Errorf("execute pipe %d: %w", i+1, err)
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
		close(errChan)
	}()
	select {
	case err = <-errChan:
		return err
	case <-done:
		return nil
	}
}

func (i *Interactor) executeMain(ctx context.Context, command string, out *os.File) error {
	args, err := shlex.Split(command)
	if err != nil {
		return fmt.Errorf("split command: %w", err)
	}
	if len(args) == 0 {
		return parser.ErrInvalidUsage
	}
	f := features.ServerFeatures{Session: i.Session}
	if out.Fd() != os.Stdout.Fd() {
		defer out.Close()
		f.Out = out
	}
	switch args[0] {
	case "c", "connect":
		return i.connect(ctx, args, out)
	case "disconnect":
		return i.disconnect(ctx, out)
	case "s", "status":
		return i.showStatus(out)
	case "T", "tools":
		return f.PrintTools(ctx)
	case "P", "prompts":
		return f.PrintPrompts(ctx)
	case "R", "resources":
		return f.PrintResources(ctx)
	case "t", "tool":
		return i.callTool(ctx, f, args)
	case "p", "prompt":
		return i.getPrompt(ctx, f, args)
	case "r", "resource":
		return i.readResource(ctx, f, args)
	case "cat":
		return i.readFile(out, args)
	case "cd":
		return i.chdir(args)
	case "clear", "cls":
		fmt.Print("\033[H\033[2J")
		return nil
	case "exit", "quit":
		os.Exit(0)
		return nil
	case "h", "help":
		printUsage()
		return nil
	case "ls":
		return i.listDir(out, args)
	case "pwd":
		return i.printPwd(out)
	case "v", "version":
		fmt.Fprintln(out, version.Short())
		return nil
	default:
		return parser.ErrInvalidUsage
	}
}

func (i *Interactor) executePipe(ctx context.Context, pipePart string, in *os.File, out *os.File) error {
	defer in.Close()
	if out.Fd() != os.Stdout.Fd() {
		defer out.Close()
	}
	pipeArgs, err := shlex.Split(pipePart)
	if err != nil {
		return fmt.Errorf("split pipe command: %w", err)
	}
	if len(pipeArgs) == 0 {
		return ErrInvalidPipe
	}
	command := exec.CommandContext(ctx, pipeArgs[0], pipeArgs[1:]...)
	command.Stdin = in
	command.Stdout = out
	command.Stderr = os.Stderr
	return command.Run()
}

func (i *Interactor) chdir(args []string) error {
	dir := "."
	if len(args) > 1 {
		dir = args[1]
	}
	return os.Chdir(dir)
}

func (i *Interactor) listDir(out *os.File, args []string) error {
	dir := "."
	for _, arg := range args[1:] {
		if !strings.HasPrefix(arg, "-") {
			dir = arg
			break
		}
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}
	for _, item := range items {
		if item.IsDir() {
			fmt.Fprintf(out, "%s/\n", item.Name())
			continue
		}
		fmt.Fprintf(out, "%s\n", item.Name())
	}
	return nil
}

func (i *Interactor) printPwd(out *os.File) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	fmt.Fprintln(out, dir)
	return nil
}

func (i *Interactor) readFile(out *os.File, args []string) error {
	if len(args) < 2 {
		return parser.ErrInvalidUsage
	}
	file, err := os.Open(args[1])
	if err != nil {
		return fmt.Errorf("open file %s: %w", args[1], err)
	}
	defer file.Close()
	if _, err := io.Copy(out, file); err != nil {
		return fmt.Errorf("read file %s: %w", args[1], err)
	}
	return nil
}

func (i *Interactor) callTool(ctx context.Context, f features.ServerFeatures, args []string) error {
	if len(args) < 2 {
		return parser.ErrInvalidUsage
	}

	flags := flag.NewFlagSet(args[1], flag.ContinueOnError)
	arguments := map[string]any{}
	tools, err := f.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	for _, tool := range tools {
		if tool.Name != args[1] {
			continue
		}
		flags.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", tool.Name)
			fmt.Fprintf(os.Stderr, "%s\n\n", tool.Description)
			fmt.Fprintln(os.Stderr, "Options:")
			flags.PrintDefaults()
		}
		for prop, v := range tool.InputSchema.Properties {
			p := new(string)
			arguments[prop] = p
			if slices.Contains(tool.InputSchema.Required, prop) {
				v.Description = fmt.Sprintf("%s (required)", cmp.Or(v.Description, v.Title))
			} else {
				v.Description = fmt.Sprintf("%s (optional)", cmp.Or(v.Description, v.Title))
			}
			flags.StringVar(p, prop, "", v.Description)
		}
	}
	if err := flags.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}
	return f.CallTool1(ctx, args[1], arguments)
}

func (i *Interactor) getPrompt(ctx context.Context, f features.ServerFeatures, args []string) error {
	if len(args) < 2 {
		return parser.ErrInvalidUsage
	}

	flags := flag.NewFlagSet(args[1], flag.ContinueOnError)
	arguments := map[string]*string{}

	prompts, err := f.ListPrompts(ctx)
	if err != nil {
		return fmt.Errorf("list prompts: %w", err)
	}
	for _, prompt := range prompts {
		if prompt.Name != args[1] {
			continue
		}
		flags.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", prompt.Name)
			fmt.Fprintf(os.Stderr, "%s\n\n", prompt.Description)
			fmt.Fprintln(os.Stderr, "Options:")
			flags.PrintDefaults()
		}
		for _, prop := range prompt.Arguments {
			p := new(string)
			arguments[prop.Name] = p
			if prop.Required {
				prop.Description = fmt.Sprintf("%s (required)", prop.Description)
			} else {
				prop.Description = fmt.Sprintf("%s (optional)", prop.Description)
			}
			flags.StringVar(p, prop.Name, "", prop.Description)
		}
	}
	if err := flags.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}
	params := map[string]string{}
	for k, v := range arguments {
		if v != nil {
			params[k] = *v
		}
	}
	return f.GetPrompt1(ctx, args[1], params)
}

func (i *Interactor) readResource(ctx context.Context, f features.ServerFeatures, args []string) error {
	if len(args) < 2 {
		return parser.ErrInvalidUsage
	}
	return f.ReadResource(ctx, args[1])
}

func (i *Interactor) connect(ctx context.Context, args []string, out *os.File) error {
	if len(args) < 2 {
		return parser.ErrInvalidUsage
	}

	parsed := parser.Parser{}
	if err := parsed.Parse(args[1:]); err != nil {
		return fmt.Errorf("parse transport args: %w", err)
	}
	parsed.Silent = true
	clientTransport, err := transport.Transport(parsed)
	if err != nil {
		return fmt.Errorf("transport: %w", err)
	}
	client := mcp.NewClient("mcpcurl", version.Short(), nil)
	session, err := client.Connect(ctx, clientTransport)
	if err != nil {
		return fmt.Errorf("connect mcp server: %w", err)
	}
	if i.Session != nil {
		i.Session.Close()
	}
	i.Session = session
	i.completer.s.Session = session
	return i.showStatus(out)
}

func (i *Interactor) disconnect(_ context.Context, out *os.File) error {
	if i.Session == nil {
		return nil
	}
	i.Session.Close()
	i.Session = nil
	return i.showStatus(out)
}

func (i *Interactor) showStatus(out *os.File) error {
	status := features.ErrNoSession.Error()
	if i.Session != nil {
		sessionID := i.Session.ID()
		if sessionID != "" {
			status = fmt.Sprintf("Connected (%s)", sessionID)
		} else {
			status = "connected"
		}
	}
	json.NewEncoder(out).Encode(map[string]string{"status": status})
	return nil
}

func printUsage() {
	fmt.Println(`Usage:
  connect <mcp_server>    Connect to mcp server
  disconnect              Disconnect from mcp server
  status                  Show connection status
  tools                   List tools
  prompts                 List prompts
  resources               List resources
  tool <name> [options]   Call tool
  prompt <name> [options] Get prompt
  resource <name>         Read resource

  cat <file>              Read file
  cd [dir]                Change working directory
  clear                   Clear the screen
  exit       	          Exit the interactor
  help                    Show this help message
  ls [dir]                List files in directory
  pwd                     Print current working directory
  version                 Show version information

Supports command pipelining and stdout redirection:
  tools | jq .name > tools.txt`)
}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func historyFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mcpurl_history")
}
