package interactor

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/cherrydra/mcpurl/features"
	"github.com/chzyer/readline"
	"github.com/google/shlex"
)

var (
	_ readline.AutoCompleter = (*mcpurlCompleter)(nil)
)

type mcpurlCompleter struct {
	ctx context.Context
	s   *features.ServerFeatures

	once      sync.Once
	completer *readline.PrefixCompleter
}

func (c *mcpurlCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	c.once.Do(func() {
		c.completer = readline.NewPrefixCompleter(
			readline.PcItem("tools"),
			readline.PcItem("prompts"),
			readline.PcItem("resources"),
			readline.PcItem("tool", readline.PcItemDynamic(
				c.listTools,
				readline.PcItemDynamic(searchFiles)),
			),
			readline.PcItem("prompt", readline.PcItemDynamic(
				c.listPrompts,
				readline.PcItemDynamic(searchFiles)),
			),
			readline.PcItem("resource", readline.PcItemDynamic(
				c.listResources),
			),
			readline.PcItem("cat"),
			readline.PcItem("cd"),
			readline.PcItem("clear"),
			readline.PcItem("connect"),
			readline.PcItem("disconnect"),
			readline.PcItem("exit"),
			readline.PcItem("help"),
			readline.PcItem("ls"),
			readline.PcItem("pwd"),
			readline.PcItem("status"),
			readline.PcItem("version"),
		)
	})
	return c.completer.Do(line, pos)
}

func (c *mcpurlCompleter) listTools(prefix string) (ret []string) {
	args, _ := shlex.Split(prefix)
	tools, err := c.s.ListTools(c.ctx)
	if err != nil {
		return nil
	}
	for _, tool := range tools {
		if len(args) > 1 && !strings.HasPrefix(tool.Name, args[1]) {
			continue
		}
		ret = append(ret, tool.Name)
	}
	return
}

func (c *mcpurlCompleter) listPrompts(prefix string) (ret []string) {
	args, _ := shlex.Split(prefix)
	prompts, err := c.s.ListPrompts(c.ctx)
	if err != nil {
		return nil
	}
	for _, prompt := range prompts {
		if len(args) > 1 && !strings.HasPrefix(prompt.Name, args[1]) {
			continue
		}
		ret = append(ret, prompt.Name)
	}
	return
}

func (c *mcpurlCompleter) listResources(prefix string) (ret []string) {
	args, _ := shlex.Split(prefix)
	resources, err := c.s.ListResources(c.ctx)
	if err != nil {
		return nil
	}
	for _, resource := range resources {
		if len(args) > 1 && !strings.HasPrefix(resource.Name, args[1]) {
			continue
		}
		ret = append(ret, resource.Name)
	}
	return
}

func searchFiles(s string) (ret []string) {
	args, _ := shlex.Split(s)
	if len(args) <= 2 || !strings.HasPrefix(args[2], "@") {
		return nil
	}

	files, err := os.ReadDir(".")
	if err != nil {
		return nil
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ret = append(ret, "@"+file.Name())
	}
	return
}
