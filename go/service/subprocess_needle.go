package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/needleworker"
)

type NeedleRouteInference interface {
	Generate(context.Context, string, string, needle.GenerationOptions) (string, error)
}

type inProcessNeedleClient struct {
	Router    *needle.Router
	Tokenizer *needle.Tokenizer
}

func (c inProcessNeedleClient) Generate(ctx context.Context, query, tools string, options needle.GenerationOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return c.Router.Generate(c.Tokenizer, query, tools, options, nil)
}

type subprocessNeedleClient struct {
	WorkerPath, ModelPath, TokenizerPath string
	Timeout                              time.Duration
	command                              func(context.Context, string, ...string) *exec.Cmd
	run                                  func(context.Context, []byte) ([]byte, []byte, error)
}

func LoadSubprocessNeedleClient(config NeedleRouterConfig) (*subprocessNeedleClient, error) {
	return loadSubprocessNeedleClient(config, needle.ReadFP32Info, needle.VerifyFP32)
}
func loadSubprocessNeedleClient(config NeedleRouterConfig, info func(string) (needle.FP32Info, error), verify func(string) error) (*subprocessNeedleClient, error) {
	if config.WorkerMode != "subprocess" {
		return nil, errors.New("Needle subprocess client requires subprocess mode")
	}
	if _, err := info(config.FP32ModelPath); err != nil {
		return nil, err
	}
	if err := verify(config.FP32ModelPath); err != nil {
		return nil, err
	}
	return &subprocessNeedleClient{WorkerPath: config.WorkerPath, ModelPath: config.FP32ModelPath, TokenizerPath: config.TokenizerPath, Timeout: time.Duration(config.WorkerTimeoutSeconds * float64(time.Second)), command: exec.CommandContext}, nil
}
func (c *subprocessNeedleClient) Generate(ctx context.Context, query, tools string, options needle.GenerationOptions) (string, error) {
	if err := needleworker.ValidateRequest(needleworker.Request{Query: query, Tools: tools, Options: options}); err != nil {
		return "", err
	}
	var input bytes.Buffer
	_ = needleworker.WriteFrame(&input, needleworker.Request{Query: query, Tools: tools, Options: options})
	work, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	var stdout, stderr []byte
	var err error
	if c.run != nil {
		stdout, stderr, err = c.run(work, input.Bytes())
	} else {
		command := c.command(work, c.WorkerPath, c.ModelPath, c.TokenizerPath)
		command.Stdin = &input
		var output, errorsOutput bytes.Buffer
		command.Stdout, command.Stderr = &output, &errorsOutput
		err = command.Run()
		stdout, stderr = output.Bytes(), errorsOutput.Bytes()
	}
	if err != nil {
		if work.Err() != nil {
			return "", fmt.Errorf("Needle worker failed: %w", work.Err())
		}
		return "", fmt.Errorf("Needle worker failed: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	reader := bytes.NewReader(stdout)
	var response needleworker.Response
	if err = needleworker.ReadFrame(reader, &response, needleworker.DefaultMaxFrameBytes); err != nil {
		return "", fmt.Errorf("Needle worker returned an invalid frame: %w", err)
	}
	if reader.Len() != 0 {
		return "", errors.New("Needle worker returned trailing frame bytes")
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "Needle worker failed"
		}
		return "", errors.New(response.Error)
	}
	return response.Output, nil
}
