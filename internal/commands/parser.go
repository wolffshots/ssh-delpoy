package commands

import (
	"fmt"
	"regexp"
	"slices"
)

type Action string

const (
	ActionDeploy  Action = "deploy"
	ActionDestroy Action = "destroy"
	ActionPS      Action = "ps"
	ActionLogs    Action = "logs"
)

var serviceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

type Request struct {
	Action  Action
	Service string
}

type Parser struct {
	allowedLogServices map[string]struct{}
}

func NewParser(allowedLogServices map[string]struct{}) Parser {
	cloned := make(map[string]struct{}, len(allowedLogServices))
	for service := range allowedLogServices {
		cloned[service] = struct{}{}
	}

	return Parser{allowedLogServices: cloned}
}

func (p Parser) Parse(args []string) (Request, error) {
	if len(args) == 0 {
		return Request{}, fmt.Errorf("no command received; allowed: deploy | ps | logs <service>")
	}

	if slices.Equal(args, []string{"docker", "compose", "pull", "&&", "docker", "compose", "up"}) {
		return Request{Action: ActionDeploy}, nil
	}

	if slices.Equal(args, []string{"docker", "compose", "ps"}) {
		return Request{Action: ActionPS}, nil
	}

	if len(args) == 4 && args[0] == "docker" && args[1] == "compose" && args[2] == "logs" {
		return p.buildLogsRequest(args[3])
	}

	switch args[0] {
	case string(ActionDeploy):
		if len(args) != 1 {
			return Request{}, fmt.Errorf("deploy does not accept arguments")
		}
		return Request{Action: ActionDeploy}, nil
	case string(ActionDestroy):
		if len(args) != 1 {
			return Request{}, fmt.Errorf("destroy does not accept arguments")
		}
		return Request{Action: ActionDestroy}, nil
	case string(ActionPS):
		if len(args) != 1 {
			return Request{}, fmt.Errorf("ps does not accept arguments")
		}
		return Request{Action: ActionPS}, nil
	case string(ActionLogs):
		if len(args) != 2 {
			return Request{}, fmt.Errorf("logs requires exactly one service name")
		}
		return p.buildLogsRequest(args[1])
	default:
		return Request{}, fmt.Errorf("command is not allowed; allowed: deploy | destroy | ps | logs <service>")
	}
}

func (p Parser) buildLogsRequest(service string) (Request, error) {
	if !serviceNamePattern.MatchString(service) {
		return Request{}, fmt.Errorf("invalid service name %q", service)
	}
	if len(p.allowedLogServices) == 0 {
		return Request{}, fmt.Errorf("logs is disabled because ALLOWED_LOG_SERVICES is empty")
	}
	if _, ok := p.allowedLogServices[service]; !ok {
		return Request{}, fmt.Errorf("service %q is not allowlisted", service)
	}

	return Request{Action: ActionLogs, Service: service}, nil
}
