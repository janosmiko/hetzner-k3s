package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"hetzner-k3s/internal/logger"
)

type Client struct {
	logger      *logger.Logger
	printOutput bool
	envVars     []string
}

func NewClient(l *logger.Logger) *Client {
	return &Client{
		logger: l,
	}
}

type Opt func(*Client)

func WithPrintOutput(printOutput bool) Opt {
	return func(c *Client) {
		c.printOutput = printOutput
	}
}

func WithEnv(env string) Opt {
	return func(c *Client) {
		c.envVars = append(c.envVars, env)
	}
}

func (c *Client) RunCommand(command string, opts ...Opt) (err error) {
	for _, opt := range opts {
		opt(c)
	}

	cmd := exec.Command("/bin/bash")
	cmd.Args = append(
		cmd.Args,
		"-c",
		command,
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, c.envVars...)

	c.logger.Debug("Local command: " + strings.Join(cmd.Args, " "))

	var (
		wg  sync.WaitGroup // nolint: varnamelen
		out []string
	)

	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	cmd.Stderr = cmd.Stdout
	outScanner := bufio.NewScanner(cmdOut)

	wg.Add(1)

	go func(printOutput bool) {
		for outScanner.Scan() {
			cx := []byte(outScanner.Text())
			out = append(out, string(cx))

			if printOutput {
				c.logger.Info(string(cx))
			}
		}

		wg.Done()
	}(c.printOutput)

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting command: %w", err)
	}

	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok { // nolint: errorlint
			// The program has exited with an exit code != 0
			if st, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				c.logger.Error("Command output: " + strings.Join(out, "\n"))
				c.logger.Error("Exit code: " + err.Error())

				return fmt.Errorf(
					"shell command exited with error: %d, output: %s", st.ExitStatus(),
					strings.Join(out, " "),
				)
			}
		} else {
			return fmt.Errorf("command not finished: %w", err)
		}
	}

	return nil
}
