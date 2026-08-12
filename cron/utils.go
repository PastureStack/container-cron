package cron

import (
	"strings"
	"time"
)

func getDuration(i int) time.Duration {
	return time.Duration(i) * time.Second
}

func getStackNameFromLabels(labels map[string]string) string {
	if stackName, ok := labels["io.pasturestack.stack.name"]; ok {
		return stackName
	}

	return ""
}

func getServiceNameFromLabels(labels map[string]string) string {
	if stackServiceName, ok := labels["io.pasturestack.project_service.name"]; ok {
		serviceSplit := strings.SplitN(stackServiceName, "/", 2)
		if len(serviceSplit) == 2 {
			return serviceSplit[1]
		}
	}
	return ""
}
