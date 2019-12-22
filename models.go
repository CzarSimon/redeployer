package main

import (
	"os/exec"
	"strings"
)

// Config service configuration
type Config struct {
	Authentication AuthKey           `yaml:"authentication,omitempty"`
	Services       map[string]Target `yaml:"services,omitempty"`
}

// AuthKey authentication key
type AuthKey struct {
	Key  string `yaml:"key,omitempty"`
	Salt string `yaml:"salt,omitempty"`
}

// Target defines a script to be run by a webhook trigger.
type Target struct {
	ID        string `yaml:"id,omitempty"`
	Binary    string `yaml:"binary,omitempty"`
	Script    string `yaml:"script,omitempty"`
	MustMatch string `yaml:"mustMatch,omitempty"`
}

func (t Target) execute(ctx *Context, args ...string) (string, error) {
	allArgs := make([]string, len(args)+1)
	allArgs[0] = t.Script
	for i, arg := range args {
		allArgs[i+1] = arg
	}

	rawOutput, err := exec.CommandContext(ctx, t.Binary, allArgs...).CombinedOutput()
	output := string(rawOutput)
	if strings.HasSuffix(output, "\n") {
		output = output[:len(output)-1]
	}

	return output, err
}

// RedeploymentRequest request body for redeployments.
type RedeploymentRequest struct {
	Target string `json:"target,omitempty"`
	Image  string `json:"image,omitempty"`
}

// ResponseMessage response containing a string message.
type ResponseMessage struct {
	Message string `json:"message,omitempty"`
}
