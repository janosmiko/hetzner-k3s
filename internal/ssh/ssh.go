package ssh

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/melbahja/goph"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"hetzner-k3s/internal/logger"
)

type Client struct {
	logger        *logger.Logger
	user          string
	keyfile       string
	passphrase    string
	timeout       int
	printOutput   bool
	verifyHostKey bool
}

type Opt func(*Client)

func WithPassphrase(passphrase string) Opt {
	return func(c *Client) {
		c.passphrase = passphrase
	}
}

func WithTimeout(timeout int) Opt {
	return func(c *Client) {
		c.timeout = timeout
	}
}

func WithPrintOutput(printoutput bool) Opt {
	return func(c *Client) {
		c.printOutput = printoutput
	}
}

func WithVerifyHostKey(verifyhostkey bool) Opt {
	return func(c *Client) {
		c.verifyHostKey = verifyhostkey
	}
}

func NewClient(logger *logger.Logger, user, keyfile string, opts ...Opt) *Client {
	c := &Client{
		logger:  logger,
		user:    user,
		keyfile: keyfile,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) SetPassphrase(passphrase string) *Client {
	c.passphrase = passphrase

	return c
}

func (c *Client) SetTimeout(timeout int) *Client {
	c.timeout = timeout

	return c
}

func (c *Client) SetPrintOutput(printoutput bool) *Client {
	c.printOutput = printoutput

	return c
}

func (c *Client) SetVerifyHostKey(verifyhostkey bool) *Client {
	c.verifyHostKey = verifyhostkey

	return c
}

func (c *Client) SSHCommand(address, command string) (result string, err error) {
	c.logger.Sugar().Debugf("SSH address: %s", address)
	c.logger.Sugar().Debugf("SSH command: %s", command)

	auth, err := goph.Key(c.keyfile, c.passphrase)
	if err != nil {
		return "", fmt.Errorf("ssh key and passphrase mismatch: %w", err)
	}

	var client *goph.Client
	if c.verifyHostKey {
		client, err = goph.NewConn(
			&goph.Config{
				Auth:     auth,
				User:     c.user,
				Addr:     address,
				Port:     22,
				Callback: VerifyHost,
			},
		)
	} else {
		client, err = goph.NewUnknown(c.user, address, auth)
	}

	if err != nil {
		return "", fmt.Errorf("ssh client error: %w", err)
	}

	defer client.Close()

	var out []byte

	o := func() error {
		out, err = client.Run(command)
		if err != nil {
			return fmt.Errorf("command failed on server %s: %w, output: %s", address, err, out)
		}

		return nil
	}

	err = retry.Do(o, retry.Attempts(3), retry.Delay(5*time.Second))
	if err != nil {
		return string(out), fmt.Errorf("failed after 3 attempts. error: %w", err)
	}

	outTxt := string(out)
	if c.printOutput {
		fmt.Println(outTxt)
	}

	return outTxt, nil
}

func ParseSSHPasshprase(privkeyPath string) (string, error) {
	privkey, err := os.ReadFile(privkeyPath)
	if err != nil {
		return "", fmt.Errorf("cannot read private key: %w", err)
	}

	var (
		passphrase     []byte
		errPassMissing = ssh.PassphraseMissingError{}
	)

	_, err = ssh.ParsePrivateKey(privkey)
	if err != nil && err.Error() == errPassMissing.Error() {
		fmt.Print("Enter SSH Passhphrase: ")

		passphrase, err = term.ReadPassword(int(syscall.Stdin)) // nolint: unconvert
		if err != nil {
			return "", fmt.Errorf("cannot read passphrase: %w", err)
		}

		fmt.Println()

		_, err = ssh.ParsePrivateKeyWithPassphrase(privkey, passphrase)
		if err != nil {
			return "", fmt.Errorf("passphrase doesn't match private key: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("unknown error: %w", err)
	}

	return string(passphrase), nil
}

func VerifyHost(host string, remote net.Addr, key ssh.PublicKey) error {
	hostFound, err := goph.CheckKnownHost(host, remote, key, "")
	if hostFound && err != nil {
		return fmt.Errorf("cannot check known hosts: %w", err)
	}

	if hostFound && err == nil {
		return nil
	}

	if !askIsHostTrusted(host, key) {
		return errors.New("you typed no, aborting")
	}

	err = goph.AddKnownHost(host, remote, key, "")
	if err != nil {
		return fmt.Errorf("cannot add known hosts: %w", err)
	}

	return nil
}

func askIsHostTrusted(host string, key ssh.PublicKey) bool {
	if viper.GetBool("auto_approve") {
		return true
	}

	fmt.Printf("Unknown Host: %s \nFingerprint: %s \n", host, ssh.FingerprintSHA256(key))
	fmt.Print("Would you like to add it? type yes or no: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
